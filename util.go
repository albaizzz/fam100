package play

import (
	"math/rand"
	"strconv"
)

// NewID genate random string based on random number in base 36
func NewID() string {
	return strconv.FormatInt(rand.Int63(), 36)
}

func chanDisabled(chanID string) bool {
	return false
}
