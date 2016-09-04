package main

import "github.com/yulrizka/bot-play"

type quizRound struct{}

func newQuizRound() *quizRound {
	return &quizRound{}
}

func (q *quizRound) HandleMessage(game *play.Game, player play.Player, text string) (finished bool, err error) {
	panic("not implemented")
}

func (q *quizRound) Rank() play.Rank {
	panic("not implemented")
}

func (q *quizRound) ID() string {
	panic("not implemented")
}
