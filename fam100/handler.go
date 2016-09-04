package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/uber-go/zap"
	"github.com/yulrizka/bot"
	"github.com/yulrizka/bot-play"
)

type handler struct {
	manager *play.Manager
	log     zap.Logger
}

func (h *handler) NewRound(g *play.Game, log zap.Logger) (r play.Round, timeout time.Duration, err error) {
	round := newQuizRound()
	h.log = log.With(zap.String("module", "fam100"), zap.String("gameID", g.ID))

	return round, roundSeconds, nil
}

func (h *handler) GameQueued(g play.Game) {
	h.log.Info("game queued")
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Dimasukkan dalam antrian, anda tetap bisa `join@%s`\n", h.manager.BotName)
	fmt.Fprintf(&buf, "Game akan dimulai setelah antrian selesai dan jumlah pemain minimal %d. waktu anrian rata-rata %0.2f", play.GameQuorum, h.manager.WaitingAvg().Seconds())
	g.Send(buf.String(), bot.Markdown)
}

func (h *handler) NotifyReadyAfterQueue(g *play.Game) {
	text := fmt.Sprintf("Game akan dimulai setelah antrian selesai")
	g.Send(text, bot.Markdown)
}

func (h *handler) NotifyUserJoined(g *play.Game, timeLeft time.Duration) {
	playersName := make([]string, len(g.Players))
	need := play.GameQuorum - len(playersName)
	if need < 0 {
		return
	}
	text := fmt.Sprintf("ok %s, butuh %d orang lagi. Sisa waktu %s", bot.TelegramEscape(strings.Join(playersName, ", ")), need, timeLeft)
	g.Send(text, bot.Markdown)
}

func (h *handler) GameStarted(g *play.Game) {
	// noop
}

func (h *handler) RoundStarted(g *play.Game, r play.Round) {
	panic("not implemented")
}

func (h *handler) RoundTimeLeft(g *play.Game, d time.Duration) {
	panic("not implemented")
}

func (h *handler) RoundFinished(g *play.Game, r play.Round, timeout bool) {
	panic("not implemented")
}

func (h *handler) GameFinished(g *play.Game, timeout bool) {
	panic("not implemented")
}
