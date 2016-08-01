package fam100

import (
	"math/rand"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/uber-go/zap"
)

var (
	RoundDuration        = 90 * time.Second
	tickDuration         = 10 * time.Second
	DelayBetweenRound    = 5 * time.Second
	TickAfterWrongAnswer = false
	RoundPerGame         = 3
	log                  zap.Logger

	playerActiveMap = cache.New(5*time.Minute, 30*time.Second)
)

func init() {
	log = zap.NewJSON()
	go func() {
		for range time.Tick(30 * time.Second) {
			playerActive.Update(int64(playerActiveMap.ItemCount()))
		}
	}()
}

func SetLogger(l zap.Logger) {
	log = l.With(zap.String("module", "fam100"))
}

type Round interface {
	SetState(s State)
	Finished() bool
	HandleMessage(chanID string, player Player, text string)
	Rank() Rank
}

type Provider interface {
	NewRound(chanID string, players map[string]Player) (Round, error)
	GameStarted(chanID string, g Game)
	RoundStarted(chanID string, g Game, r Round)
	RoundFinished(chanID string, g Game, r Round, timeout bool)
	GameFinished(chanID string, g Game)
	DisplayTimeLeft(chanID string, d time.Duration)
	DisplayAnswer(chanID string, r Round)
}

// Message to communicate between player and the game
type Message interface{}

// TextMessage represents a chat message
type TextMessage struct {
	ChanID     string
	Player     Player
	Text       string
	ReceivedAt time.Time
}

// StateMessage represents state change in the game
type StateMessage struct {
	GameID    int64
	ChanID    string
	Round     int
	State     State
	RoundText QNAMessage //question and answer
}

// TickMessage represents time left notification
type TickMessage struct {
	ChanID   string
	TimeLeft time.Duration
}

type WrongAnswerMessage TickMessage

// QNAMessage represents question and answer for a round
type QNAMessage struct {
	ChanID         string
	Round          int
	QuestionText   string
	QuestionID     int
	Answers        []roundAnswers
	ShowUnanswered bool // reveal un-answered question (end of round)
	TimeLeft       time.Duration
}

type roundAnswers struct {
	Text       string
	Score      int
	Answered   bool
	PlayerName string
	Highlight  bool
}
type RankMessage struct {
	ChanID string
	Round  int
	Rank   Rank
	Final  bool
}

// Player of the game
type Player struct {
	ID, Name string
}

// State represents state of the round
type State string

// Available state
// TODO: simplify enum
const (
	Created       State = "created"
	Started       State = "started"
	Finished      State = "finished"
	RoundStarted  State = "roundStarted"
	RoundTimeout  State = "RoundTimeout"
	RoundFinished State = "roundFinished"
)

// Game can consists of multiple round
// each round user will be asked question and gain points
type Game struct {
	ID           int64
	ChanID       string
	ChanName     string
	State        State
	RoundCount   int
	players      map[string]Player
	rank         Rank
	currentRound Round
	p            Provider

	In chan Message
}

// NewGame create a new round
func NewGame(chanID, chanName string, in chan Message, p Provider) (r *Game, err error) {

	return &Game{
		ID:       int64(rand.Int31()),
		ChanID:   chanID,
		ChanName: chanName,
		State:    Created,
		players:  make(map[string]Player),
		In:       in,
	}, err
}

// Start the game
func (g *Game) Start() {
	g.State = Started

	go func() {
		g.p.GameStarted(g.ChanID, *g)

		for i := 1; i <= RoundPerGame; i++ {
			err := g.startRound(i)
			if err != nil {
				log.Error("starting round failed", zap.String("chanID", g.ChanID), zap.Error(err))
			}
			final := i == RoundPerGame
			if !final {
				time.Sleep(DelayBetweenRound)
			}
		}
		g.State = Finished
		log.Info("Game finished", zap.String("chanID", g.ChanID), zap.Int64("gameID", g.ID))
		g.p.GameFinished(g.ChanID, *g)
	}()
}

func (g *Game) startRound(currentRound int) error {
	r, _ := g.p.NewRound(g.ChanID, g.players)

	g.currentRound = r
	r.SetState(RoundStarted)
	timeUp := time.After(RoundDuration)
	timeLeftTick := time.NewTicker(tickDuration)
	displayAnswerTick := time.NewTicker(tickDuration)

	g.p.RoundStarted(g.ChanID, *g, r)

	for {
		select {
		case rawMsg := <-g.In: // new answer coming from player
			started := time.Now()
			msg, ok := rawMsg.(TextMessage)
			if !ok {
				log.Error("Unexpected message type input from client")
				continue
			}
			gameLatencyTimer.UpdateSince(msg.ReceivedAt)
			playerActiveMap.Set(string(msg.Player.ID), struct{}{}, cache.DefaultExpiration)

			r.HandleMessage(msg.ChanID, msg.Player, msg.Text) //TODO: handle receivedAt or convert it to bot.Message

			if r.Finished() {
				timeLeftTick.Stop()
				displayAnswerTick.Stop()

				r.SetState(RoundFinished)
				g.updateRanking(r.Rank())
				g.p.RoundFinished(g.ChanID, *g, r, false)

				gameMsgProcessTimer.UpdateSince(started)
				gameServiceTimer.UpdateSince(msg.ReceivedAt)

				return nil
			}

			gameMsgProcessTimer.UpdateSince(started)
			gameServiceTimer.UpdateSince(msg.ReceivedAt)

		case <-timeLeftTick.C:
			timeLeft := time.Second // TODO: move  timeleft tracing to game
			g.p.DisplayTimeLeft(g.ChanID, timeLeft)

		case <-displayAnswerTick.C:
			// show correct answer (at most once every 10s)
			g.p.DisplayAnswer(g.ChanID, r)

		case <-timeUp:
			timeLeftTick.Stop()
			displayAnswerTick.Stop()

			g.State = RoundFinished
			g.updateRanking(r.Rank())
			g.p.RoundFinished(g.ChanID, *g, r, true)

			return nil
		}
	}
}

func (g *Game) updateRanking(r Rank) {
	g.rank = g.rank.Add(r)
	DefaultDB.saveScore(g.ChanID, g.ChanName, r)
}
