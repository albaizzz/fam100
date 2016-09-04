package play

import (
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/uber-go/zap"
	"github.com/yulrizka/bot"
)

var (
	gameQuorum     = 3
	maxActiveGame  = 400
	quorumDuration = 120 * time.Second
	roundPerGame   = 3
	log            zap.Logger
)

type state string

const (
	// game is in the queue
	queued state = "queued"
	// game is ready to be started (also possible still in the queue)
	ready state = "ready"
	// game is started
	started state = "started"
	// game is finished
	finished state = "finished"
)

// Player of a game
type Player struct {
	ID, Name, Username string
}

// Chan represent a channel (group) chat
type Chan struct {
	ID, Name string
}

// Round represent a single round to be played within a game
type Round interface {
	ID() string
	HandleMessage(game *Game, player Player, text string) (finished bool, err error)
	Rank() Rank
}

// Manager of the whole game. It process incomming message and route to specific game,
// handle commands, start and manage games
type Manager struct {
	BotName string
	games   map[string]*Game
	handler Handler
	log     zap.Logger

	finishedCh chan string   // signal finished game
	tokenQueue chan struct{} // token that each game needs to aquire before starting the game

	QueueTimer metrics.Timer
}

// NewManager create a Manager for given client with a game handler
func NewManager(client bot.Client, handler Handler) *Manager {
	return &Manager{
		BotName:    client.Username(),
		games:      make(map[string]*Game),
		handler:    handler,
		tokenQueue: make(chan struct{}, maxActiveGame),
		log:        log.With(zap.String("module", "manager"), zap.String("botName", "botName")),
	}
}

// Process incomming message
func (m *Manager) Process(msg *bot.Message) error {
	chanID := msg.Chat.ID

	g, ok := m.games[chanID]
	if msg.Text == "/join" || msg.Text == "/join@"+m.BotName {
		if chanDisabled(chanID) {
			return nil

		}

		p := Player{ID: msg.ID, Name: msg.From.FullName(), Username: msg.From.Username}
		if !ok {
			// no game has been started yet
			g = newGame(Chan{ID: chanID, Name: msg.Chat.Title}, m.handler, m.finishedCh, m.tokenQueue, m.log)
			g.Players[p.ID] = p
			m.games[chanID] = g
		}
		select {
		case g.joinCh <- p:
		default:
			m.log.Warn("joinCh full", zap.String("chanID", chanID))
		}
		return nil
	}
	select {
	case g.msgCh <- msg:
	default:
		m.log.Warn("msgCh full", zap.String("chanID", chanID))
	}
	return nil
}

// WaitingAvg return duration of waiting for queued game
func (m *Manager) WaitingAvg() time.Duration {
	return time.Duration(int64(m.QueueTimer.Mean()))
}
