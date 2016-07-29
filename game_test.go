package fam100

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	redisPrefix = "test_fam100"
	DefaultDB.Init()
	DefaultDB.Reset()
	retCode := m.Run()
	os.Exit(retCode)
}
