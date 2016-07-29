package fam100

import (
	"math/rand"
	"sort"
	"time"

	"github.com/uber-go/zap"
)

// round represents with one question
type round struct {
	id        int64
	q         Question
	state     State
	correct   []PlayerID // correct answer answered by a player, "" means not answered
	players   map[PlayerID]Player
	highlight map[int]bool

	endAt time.Time
}

func newRound(seed int64, totalRoundPlayed int, players map[PlayerID]Player, questionLimit int) (*round, error) {
	q, err := NextQuestion(seed, totalRoundPlayed, questionLimit)
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
		endAt:     time.Now().Add(RoundDuration).Round(time.Second),
	}, nil
}

func (r *round) timeLeft() time.Duration {
	return r.endAt.Sub(time.Now().Round(time.Second))
}

// questionText construct QNAMessage which contains questions, answers and score
func (r *round) questionText(gameID string, showUnAnswered bool) QNAMessage {
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

	msg := QNAMessage{
		ChanID:         gameID,
		QuestionText:   r.q.Text,
		QuestionID:     r.q.ID,
		ShowUnanswered: showUnAnswered,
		TimeLeft:       r.timeLeft(),
		Answers:        ras,
	}

	return msg
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
func (r *round) ranking() Rank {
	var roundScores Rank
	lookup := make(map[PlayerID]PlayerScore)
	for i, pID := range r.correct {
		if pID != "" {
			score := r.q.Answers[i].Score
			if ps, ok := lookup[pID]; !ok {
				lookup[pID] = PlayerScore{
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

func (r *round) answer(p Player, text string) (correct, answered bool, index int) {
	if r.state != RoundStarted {
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

func (g *round) handleMessage(msg TextMessage, r *Round) (handled bool) {
	log.Debug("startRound got message", zap.String("chanID", g.ChanID), zap.Object("msg", msg))
	answer := msg.Text
	correct, alreadyAnswered, idx := r.answer(msg.Player, answer)
	if !correct {
		if TickAfterWrongAnswer {
			g.Out <- WrongAnswerMessage{ChanID: g.ChanID, TimeLeft: r.timeLeft()}
		}
		return true
	}
	if alreadyAnswered {
		log.Debug("already answered", zap.String("chanID", g.ChanID), zap.String("by", string(r.correct[idx])))
		return true
	}

	log.Info("answer correct",
		zap.String("playerID", string(msg.Player.ID)),
		zap.String("playerName", msg.Player.Name),
		zap.String("answer", answer),
		zap.Int("questionID", r.q.ID),
		zap.String("chanID", g.ChanID),
		zap.Int64("gameID", g.ID),
		zap.Int64("roundID", r.id))

	return false
}

func (g *Game) showAnswer(r *Round) {
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

	qnaText := r.questionText(g.ChanID, false)
	select {
	case g.Out <- qnaText:
	default:
	}

	for i := range r.highlight {
		r.highlight[i] = false
	}
}
