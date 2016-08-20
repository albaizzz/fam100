package play

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/uber-go/zap"
	"github.com/yulrizka/bot"
)

var (
	gameQuorum   = 3
	roundPerGame = 3
	log          zap.Logger
)

type state string

const (
	created  state = "created"
	queued   state = "queued"
	started  state = "started"
	finished state = "finished"
)

type Player struct {
	ID, Name, Username string
}

type Chan struct {
	ID, Name string
}

type Round interface {
	Finished() bool
	HandleMessage(chanID string, game Game, player Player, text string)
	Rank() Rank
}

type Provider interface {
	NewRound(g *Game) (Round, error)
	GameStarted(g *Game)
	RoundStarted(g *Game, r Round)
	RoundFinished(g *Game, r Round, timeout bool)
	GameFinished(g *Game)
	DisplayTimeLeft(d time.Duration)
}

type Manager struct {
	botName  string
	games    map[string]*Game
	provider Provider
}

func NewManager(botName string) *Manager {
	return &Manager{
		botName: botName,
		games:   make(map[string]*Game),
	}
}

func (m *Manager) Process(msg *bot.Message) error {
	player := Player{ID: msg.ID, Name: msg.From.FullName(), Username: msg.From.Username}

	// process join message
	if msg.Text == "/join" || msg.Text == "/join@"+m.botName {
		return m.processJoin(msg, player)
	}

	return nil
}

func (m *Manager) processJoin(msg *bot.Message, p Player) error {
	if chanDisabled() {
		return nil
	}
	chanID := msg.Chat.ID

	game, ok := m.games[chanID]
	if !ok {
		g := newGame(Chan{ID: chanID, Name: msg.Chat.Title}, m.provider)
		g.Players[p.ID] = p
		m.games[chanID] = g

		if len(g.Players) == gameQuorum {
			// TODO: start game
		}
		return nil
	}

	if _, ok = game.Players[p.ID]; ok {
		return nil
	}

	// new player
	game.Players[p.ID] = p
	if len(game.Players) == gameQuorum {
		// TODO: start game
	}

	return nil
}

// NewID genate random string based on random number in base 36
func NewID() string {
	return strconv.FormatInt(rand.Int63(), 36)
}

func chanDisabled() bool {
	return false
}
