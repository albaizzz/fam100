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

var (
	roundSeconds = 90
)

type fam100 struct {
	manager *play.Manager
	log     zap.Logger
}

func (f *fam100) NewRound(g *play.Game, log zap.Logger) (r play.Round, timeout time.Duration, err error) {
	round := newQuizRound()
	f.log = log.With(zap.String("module", "fam100"))

	return round, time.Duration(roundSeconds) * time.Second, nil
}

func (f *fam100) GameQueued(g play.Game) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Dimasukkan dalam antrian, anda tetap bisa `join@%s`\n", f.manager.BotName)
	fmt.Fprintf(&buf, "Game akan dimulai setelah antrian selesai dan jumlah pemain minimal %d. waktu anrian rata-rata %0.2f", play.GameQuorum, f.manager.WaitingAvg().Seconds())
	g.Send(buf.String(), bot.Markdown)
}

func (f *fam100) NotifyReadyAfterQueue(g *play.Game) {
	text := fmt.Sprintf("Game akan dimulai setelah antrian selesai")
	g.Send(text, bot.Markdown)
}

func (f *fam100) NotifyUserJoined(g *play.Game, timeLeft time.Duration) {
	playersName := make([]string, len(g.Players))
	need := play.GameQuorum - len(playersName)
	if need < 0 {
		return
	}
	text := fmt.Sprintf("ok %s, butuh %d orang lagi. Sisa waktu %s", bot.TelegramEscape(strings.Join(playersName, ", ")), need, timeLeft)
	g.Send(text, bot.Markdown)
}

func (f *fam100) GameStarted(g *play.Game) {
	// noop
}

func (f *fam100) RoundStarted(g *play.Game, r play.Round) {
	panic("not implemented")
}

func (f *fam100) RoundTimeLeft(g *play.Game, d time.Duration) {
	panic("not implemented")
}

func (f *fam100) RoundFinished(g *play.Game, r play.Round, timeout bool) {
	panic("not implemented")
}

func (f *fam100) GameFinished(g *play.Game, timeout bool) {
	panic("not implemented")
}
