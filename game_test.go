package fam100

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	redisPrefix = "test_fam100"
	if _, err := InitQuestion("test.db"); err != nil {
		panic(err)
	}
	DefaultDB.Init()
	DefaultDB.Reset()
	retCode := m.Run()
	os.Exit(retCode)
}
