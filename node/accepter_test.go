package paxos

import (
	"errors"
	"log"
	"os"
	"testing"
	"time"
)

var (
	/* Network parameters. */
	proposers = 2
	accepters = 3
	learners  = 5
)

func setup() func() {
	SetPersistAfterShutdown(false)
	nodes, err := CreateNetwork(proposers, accepters, learners)
	if err != nil {
		log.Fatal(err)
	}
	return func() { nodes.Close() }
}

func TestMain(m *testing.M) {
	teardown := setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func TestAccepter(t *testing.T) {
	n := time.Duration(2)
	time.Sleep(n * time.Second)
	failTest(t, errors.New("ølkajsdf"))
}

func failTest(t *testing.T, err error) {
	t.Error(err)
	t.FailNow()
}
