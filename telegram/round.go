package main

import (
	"math/rand"
	"sort"
	"time"

	"github.com/uber-go/zap"
	"github.com/yulrizka/fam100"
)

// round represents with one question
type round struct {
	id        int64
	q         Question
	state     fam100.State
	correct   []string // contains playerID that answered i-th answer, "" means not answered
	players   map[string]fam100.Player
	highlight map[int]bool

	endAt time.Time
}

type roundAnswers struct {
	Text       string
	Score      int
	Answered   bool
	PlayerName string
	Highlight  bool
}

func newRound(seed int64, totalRoundPlayed int, players map[string]fam100.Player, questionLimit int) (*round, error) {
	q, err := NextQuestion(seed, totalRoundPlayed, questionLimit)
	if err != nil {
		return nil, err
	}

	return &round{
		id:        int64(rand.Int31()),
		q:         q,
		correct:   make([]string, len(q.Answers)),
		state:     fam100.Created,
		players:   players,
		highlight: make(map[int]bool),
		endAt:     time.Now().Add(fam100.RoundDuration).Round(time.Second),
	}, nil
}

func (r *round) timeLeft() time.Duration {
	return r.endAt.Sub(time.Now().Round(time.Second))
}

// questionText construct QNAMessage which contains questions, answers and score
// TODO: remove QNAMessage
func (r *round) questionText(gameID string, showUnAnswered bool) fam100.QNAMessage {
	/*
		ras := make([]roundAnswers, len(r.q.Answers))

		for i, ans := range r.q.Answers {
			ra := roundAnswers{
				Text:  ans.String(),
				Score: ans.Score,
			}
			if pID := r.correct[i]; pID != "" {
				ra.Answered = true
				ra.PlayerName = r.players[pID].Name
			}
			if r.highlight[i] {
				ra.Highlight = true
			}
			ras[i] = ra
		}

		msg := fam100.QNAMessage{
			ChanID:         gameID,
			QuestionText:   r.q.Text,
			QuestionID:     r.q.ID,
			ShowUnanswered: showUnAnswered,
			TimeLeft:       r.timeLeft(),
			Answers:        ras,
		}
	*/

	return fam100.QNAMessage{}
}

func (r *round) finised() bool {
	answered := 0
	for _, pID := range r.correct {
		if pID != "" {
			answered++
		}
	}

	return answered == len(r.q.Answers)
}

// ranking generates a rank for current round which contains player, answers and score
func (r *round) ranking() fam100.Rank {
	var roundScores fam100.Rank
	lookup := make(map[string]fam100.PlayerScore)
	for i, pID := range r.correct {
		if pID != "" {
			score := r.q.Answers[i].Score
			if ps, ok := lookup[pID]; !ok {
				lookup[pID] = fam100.PlayerScore{
					PlayerID: pID,
					Name:     r.players[pID].Name,
					Score:    score,
				}
			} else {
				ps = lookup[pID]
				ps.Score += score
				lookup[pID] = ps
			}
		}
	}

	for _, ps := range lookup {
		roundScores = append(roundScores, ps)
	}
	sort.Sort(roundScores)
	for i := range roundScores {
		roundScores[i].Position = i + 1
	}

	return roundScores
}

func (r *round) answer(p fam100.Player, text string) (correct, answered bool, index int) {
	if r.state != fam100.RoundStarted {
		return false, false, -1
	}

	if _, ok := r.players[p.ID]; !ok {
		r.players[p.ID] = p
	}
	if correct, _, i := r.q.checkAnswer(text); correct {
		if r.correct[i] != "" {
			// already answered
			return correct, true, i
		}
		r.correct[i] = p.ID
		r.highlight[i] = true

		return correct, false, i
	}
	return false, false, -1
}

// TODO: remove TextMessage, replace by bot.message
func (r *round) HandleMessage(chanID string, game fam100.Game, player fam100.Player, answer string) {
	log.Debug("startRound got message", zap.String("chanID", chanID), zap.Object("player", player), zap.String("answer", answer))

	correct, alreadyAnswered, _ := r.answer(player, answer)
	if !correct {
		return
	}
	if alreadyAnswered {
		log.Debug("already answered", zap.String("chanID", chanID), zap.Object("player", player))
		return
	}

	log.Info("answer correct",
		zap.String("playerID", player.ID),
		zap.String("playerName", player.Name),
		zap.String("answer", answer),
		zap.Int("questionID", r.q.ID),
		zap.String("chanID", chanID),
		zap.Int64("gameID", game.ID),
		zap.Int64("roundID", r.id))

	return
}

func (r *round) showAnswer() {
	var show bool
	// if there is no highlighted answer don't display
	for _, v := range r.highlight {
		if v {
			show = true
			break
		}
	}
	if !show {
		return
	}

	// TODO: remove this change output message directly
	/*
		qnaText := r.questionText(g.ChanID, false)
		select {
		case g.Out <- qnaText:
		default:
		}*/

	for i := range r.highlight {
		r.highlight[i] = false
	}
}

func (r *round) Finished() bool {
	return false
}

func (r *round) Rank() fam100.Rank {
	return fam100.Rank{}
}

func (r *round) SetState(s fam100.State) {
}
