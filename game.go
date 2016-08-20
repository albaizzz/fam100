package play

import "github.com/uber-go/zap"

type Game struct {
	ID       string
	Players  map[string]Player
	Chan     Chan
	state    state
	round    Round
	provider Provider
}

func newGame(ch Chan, p Provider) *Game {
	return &Game{
		ID:       NewID(),
		Players:  make(map[string]Player),
		Chan:     ch,
		state:    created,
		provider: p,
	}
}

func (g *Game) start() {
	g.state = started
	g.provider.GameStarted(g)
	for i := 0; i < roundPerGame; i++ {
		var err error
		g.round, err = g.provider.NewRound(g)
		if err != nil {
			log.Error("failed creating game", zap.String("chanID", g.Chan.ID), zap.String("gameID", g.ID))
			return
		}
		g.provider.GameStarted(g)
	}
	g.provider.GameFinished(g)
}
