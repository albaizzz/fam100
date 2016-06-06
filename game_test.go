package fam100

import (
	"math/rand"
	"testing"
)

func TestQuestionString(t *testing.T) {
	r, err := newRound(7, 0, make(map[PlayerID]Player))
	if err != nil {
		t.Error(err)
	}
	r.state = Started
	rand.Seed(7)
	players := []Player{
		Player{ID: "1", Name: "foo"},
		Player{ID: "2", Name: "bar"},
		Player{ID: "3", Name: "baz"},
	}
	idx := rand.Perm(len(r.q.answers))
	for i := 0; i < len(players); i++ {
		answerText := r.q.answers[idx[i]].text[0]
		player := players[rand.Intn(len(players))]
		r.answer(player, answerText)
	}
	// no checking, just to debug output
	/*
		showUnAnswered := false
		fmt.Print(r.questionText(showUnAnswered))
		fmt.Println()
		for pID, score := range r.scores() {
			p := r.players[pID]
			fmt.Printf("p.name = %s, score = %d\n", p.Name, score)
		}
	*/
}
