package play

import (
	"context"
	"time"

	"github.com/uber-go/zap"
	"github.com/yulrizka/bot"
)

const gameMsgBuffer = 100

var (
	//GameQuorum number of players needs to joined before the game start
	GameQuorum = 3

	notifyDuration   = 5 * time.Second
	timeLeftDuration = 30 * time.Second
)

// Handler is an interface that game provider should implement. It acts as call back when certain event happens or
// manager need some info from provider
type Handler interface {
	// NewRound will be called when managers need to create a new round to the game
	NewRound(g *Game, log zap.Logger) (r Round, timeout time.Duration, err error)

	// GameQueued called when too much game is active and this game is queued
	GameQueued(g Game)

	// NotifyReadyAfterQueue called when the game is ready to be started but waiting on queue
	NotifyReadyAfterQueue(g *Game)

	// User (could be more than 1 user) joined the game
	NotifyUserJoined(g *Game, timeLeft time.Duration)

	// Called when game is started
	GameStarted(g *Game)

	// Called when a new round is started
	RoundStarted(g *Game, r Round)

	// Called to notify how much time before the round end
	RoundTimeLeft(g *Game, d time.Duration)

	// Called when the round is finished
	RoundFinished(g *Game, r Round, timeout bool)

	// Called when the game is finished
	GameFinished(g *Game, timeout bool)
}

// Game represent a game in a channel. Games can have multiple round
type Game struct {
	ID      string
	Players map[string]Player
	Chan    Chan
	Rank    Rank
	nRound  int

	msgCh   chan *bot.Message
	state   state
	round   Round
	handler Handler
	log     zap.Logger

	ctx, quorumCtx           context.Context
	cancelGame, cancelQuorum context.CancelFunc
	quorumTimer              *time.Timer // timer wait until quorum is achieved

	joinCh   chan Player
	queueCh  chan struct{} // chan that will be closed if this game finished queueing
	notifyCh chan struct{} // notify status of the quorum

	finishedCh chan<- string // Manager's finished ch, tell manager that gameID is finished
	tokenCh    chan struct{} // Manager's token queue that game needs to get before starting and put back after finished (for queueing)
}

func newGame(ch Chan, h Handler, finishedCh chan<- string, tokenCh chan struct{}, log zap.Logger) *Game {
	g := Game{
		ID:         NewID(),
		Players:    make(map[string]Player),
		Chan:       ch,
		state:      queued,
		handler:    h,
		joinCh:     make(chan Player),
		msgCh:      make(chan *bot.Message, gameMsgBuffer),
		finishedCh: finishedCh,
		tokenCh:    tokenCh,
	}
	g.log = log.With(zap.String("module", "game"), zap.String("gameID", g.ID))

	g.ctx, g.cancelGame = context.WithCancel(context.Background())
	g.quorumCtx, g.cancelQuorum = context.WithCancel(g.ctx)

	go func() {
		// test if we are on manager's queue
		select {
		case <-tokenCh:
			close(g.queueCh)
		default:
			// we are on the queue
			g.state = queued
			g.handler.GameQueued(g)
			<-tokenCh // wait until we can play
			close(g.queueCh)
		}
	}()

	go g.startQuorum()

	return &g
}

// startQuorum waits until enough people join the game or cancels it after timeout
// TODO: this structure still seems complicated
func (g *Game) startQuorum() {
	lastNotified := time.Time{}
	for {
		select {
		case <-g.ctx.Done():
			// global quit
			return
		case <-g.quorumCtx.Done():
			// quorum cancelled
			return
		case p := <-g.joinCh:
			// user joined
			g.handleJoin(p)
		case <-g.quorumTimer.C:
			// quorum not achieved
			g.finishWithTimeout(true)
			return
		case <-g.notifyCh:
			if len(g.Players) >= GameQuorum || time.Now().Sub(lastNotified) < notifyDuration {
				continue
			}
			lastNotified = time.Now()
			switch g.state {
			case queued, ready:
				//TODO: calculate duration
				timeLeft := time.Duration(0)
				g.handler.NotifyUserJoined(g, timeLeft)
			}
		case <-g.queueCh:
			// finished queueing
			g.state = ready
			if len(g.Players) >= GameQuorum {
				g.stopQuorum()
				go g.startGame()
				return
			}
		case <-g.msgCh:
			// ignore normal message
		}
	}
}

func (g *Game) stopQuorum() {
	if g.quorumTimer != nil {
		g.quorumTimer.Stop()
	}
	g.cancelQuorum()
}

func (g *Game) finishWithTimeout(timeout bool) {
	switch g.state {
	case ready, started:
		g.tokenCh <- struct{}{} // return token back to manager so other game can start
	}
	g.state = finished
	g.handler.GameFinished(g, timeout)
	g.finishedCh <- g.ID // inform to manager the game is finished and ready to be cleaned up
}

func (g *Game) handleJoin(p Player) {
	switch g.state {
	case started, finished:
		return
	}

	if _, ok := g.Players[p.ID]; ok {
		return
	}

	g.Players[p.ID] = p
	if len(g.Players) >= GameQuorum {
		g.stopQuorum()
		switch g.state {
		case queued:
			g.handler.NotifyReadyAfterQueue(g)
		case ready:
			go g.startGame()
		}

		return
	}

	// reset the quorum timer
	if g.quorumTimer == nil {
		g.quorumTimer = time.NewTimer(quorumDuration)
	} else {
		g.quorumTimer.Reset(quorumDuration)
	}

	// only notify channel of new user joined if game is not on the queue
	if g.state != ready {
		return
	}
	g.notifyCh <- struct{}{}
}

func (g *Game) startGame() {
	g.state = started
	g.handler.GameStarted(g)

GAME:
	for i := 1; i < roundPerGame; i++ {
		g.nRound = i
		round, timeoutDuration, err := g.handler.NewRound(g, g.log)
		if err != nil {
			g.log.Error("failed to create game", zap.Error(err))
		}
		g.round = round
		gameTimer := time.NewTimer(timeoutDuration)
		timeLeftTimer := time.NewTimer(timeoutDuration - timeLeftDuration)
	ROUND:
		for {
			select {
			case <-g.ctx.Done():
				// game canceled
				break GAME

			case <-gameTimer.C:
				// timed out
				g.Rank.Add(round.Rank())
				timeout := true
				g.handler.RoundFinished(g, round, timeout)
				break ROUND

			case <-timeLeftTimer.C:
				g.handler.RoundTimeLeft(g, timeLeftDuration)

			case msg := <-g.msgCh:
				p := Player{ID: msg.ID, Name: msg.From.FullName(), Username: msg.From.Username}
				finished, err := round.HandleMessage(g, p, msg.Text)
				if err != nil {
					break GAME
				}
				if finished {
					gameTimer.Stop()
					g.Rank.Add(round.Rank())
					timeout := false
					g.handler.RoundFinished(g, round, timeout)
					break ROUND
				}
			}
		}
	}

	g.finishWithTimeout(false)
}

// Send message to the channel
func (g *Game) Send(text string, format bot.MessageFormat) {
}
