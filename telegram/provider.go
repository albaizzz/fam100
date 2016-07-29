package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"time"

	"github.com/uber-go/zap"
	"github.com/yulrizka/bot"
	"github.com/yulrizka/fam100"
)

type fam100Provider struct {
}

func (fam100Provider) NewRound(chanID string, players map[string]Player) fam100.Round {
	seed, totalRoundPlayed, err := fam100.DefaultDB.nextGame(chanID)

	questionLimit := DefaultQuestionLimit
	if limitConf, err := DefaultDB.ChannelConfig(g.ChanID, "questionLimit", ""); err == nil && limitConf != "" {
		if limit, err := strconv.ParseInt(limitConf, 10, 64); err == nil {
			questionLimit = int(limit)
		}
	}

	q, err := fam100.NextQuestion(seed, totalRoundPlayed, questionLimit)
	if err != nil {
		return nil, err
	}

	return &round{
		id:        int64(rand.Int31()),
		q:         q,
		correct:   make([]PlayerID, len(q.Answers)),
		state:     Created,
		players:   players,
		highlight: make(map[int]bool),
		endAt:     time.Now().Add(fam100.RoundDuration).Round(time.Second),
	}, nil
}

func (fam100Provider) GameStarted(chanID string, g fam100.Game) {
	gameStartedCount.Inc(1)
}
func (fam100Provider) RoundStarted(chanID string, g fam100.Game, r Round) {
	log.Info("Round Started", zap.String("chanID", ChanID), zap.Int64("gameID", g.ID), zap.Int64("roundID", r.ID), zap.Int("questionID", r.q.ID), zap.Int("questionLimit", questionLimit))

	var text string
	if g.RoundCount == 1 {
		text = fmt.Sprintf("Game (id: %d) dimulai\n<b>siapapun boleh menjawab tanpa</b> /join\n", g.ID)
	}
	roundStartedCount.Inc(1)
	text += fmt.Sprintf("Ronde %d dari %d", g.RoundCount, fam100.RoundPerGame)
	text += "\n\n" + formatRoundText(msg.RoundText)
	b.out <- bot.Message{Chat: bot.Chat{ID: msg.ChanID}, Text: text, Format: bot.HTML, Retry: 3}
	messageOutgoingCount.Inc(1)
}

func (fam100Provider) RoundFinished(chanID, g fam100.Game, r Round, timeout bool) {
	log.Info("Round finished", zap.String("chanID", g.ChanID), zap.Int64("gameID", g.ID), zap.Int64("roundID", r.ID), zap.Bool("timeout", timeout))

	roundFinishedCount.Inc(1)
	if timeout {
		roundTimeoutCount.Inc(1)
	}

	// TODO show answer wih show unanswered

	if g.RoundCount == fam100.RoundPerGame { // final round
		if msg.Final {
			text = "<b>Final score</b>:" + text

			// show leader board, TOP 3 + current game players
			rank, err := fam100.DefaultDB.ChannelRanking(msg.ChanID, 3)
			if err != nil {
				log.Error("getting channel ranking failed", zap.String("chanID", msg.ChanID), zap.Error(err))
				continue
			}
			lookup := make(map[fam100.PlayerID]bool)
			for _, v := range rank {
				lookup[v.PlayerID] = true
			}
			for _, v := range msg.Rank {
				if !lookup[v.PlayerID] {
					playerScore, err := fam100.DefaultDB.PlayerChannelScore(msg.ChanID, v.PlayerID)
					if err != nil {
						continue
					}

					rank = append(rank, playerScore)
				}
			}
			sort.Sort(rank)
			text += "\n<b>Total Score</b>" + formatRankText(rank)

			text += fmt.Sprintf("\n<a href=\"http://labs.yulrizka.com/fam100/scores.html?c=%s\">Full Score</a>\n", msg.ChanID)
			text += fmt.Sprintf("\nGame selesai!")
			motd, _ := messageOfTheDay(msg.ChanID)
			if motd != "" {
				text = fmt.Sprintf("%s\n\n%s", text, motd)
			}
		} else {
			text = "Score sementara:" + text
		}
		b.out <- bot.Message{Chat: bot.Chat{ID: msg.ChanID}, Text: text, Format: bot.HTML, Retry: 3}
		messageOutgoingCount.Inc(1)
	}
}

func (fam100Provider) GameFinished(chanID, g Game) {
	gameFinishedCount.Inc(1)
	finishedChan <- msg.ChanID
	messageOutgoingCount.Inc(1)
}

func (fam100Provider) DisplayTimeLeft(chanID, d time.Duration) {
	if d == 30*time.Second {
		text := fmt.Sprintf("sisa waktu %s", msg.TimeLeft)
		b.out <- bot.Message{Chat: bot.Chat{ID: ChanID}, Text: text, Format: bot.HTML}
		messageOutgoingCount.Inc(1)
	}
}

func (fam100Provider) DisplayAnswer(chanID) {
	text := formatRoundText(msg)

	outMsg := bot.Message{Chat: bot.Chat{ID: msg.ChanID}, Text: text, Format: bot.HTML}
	if !msg.ShowUnanswered {
		answerCorrectCount.Inc(1)
		outMsg.DiscardAfter = time.Now().Add(5 * time.Second)
	} else {
		// mesage at the end of timeout
	}
	b.out <- outMsg
	// TODOL display answer to channel
}
