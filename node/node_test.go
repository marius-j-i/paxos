package paxos

import (
	"bytes"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/marius-j-i/paxos/util"
	"github.com/stretchr/testify/assert"
)

var (
	/* Errors. */
	errWrongStatusCode = errors.New("wrong status code: expected %v, got %v")

	/* Network parameters. */
	proposers = 9
	accepters = 19
	learners  = 11
	network   = &Network{} // global reference to instanciated network
	msPerNode = 50         // ms per node in network to wait until stabilization

	/* Run-time. */
	testDirOut = "test-nodes" // output directory for testing.
	persist    = false
	// persist    = true
	emptyBody = bytes.NewReader([]byte{})
)

/* Setup network and return teardown method. */
func setup() func() {
	/* No disk access. */
	SetPersistState(persist)
	/* Disable nodes from picking up from previous nodes ran. */
	SetRestorePersistentState(false)
	/* Set output directory for testing. */
	SetNodeDirectory(testDirOut)
	/* Clean up files so different test runs do not interfere. */
	SetPersistAfterShutdown(persist)
	/* Start network. */
	N, err := NewNetwork(proposers, accepters, learners)
	if err != nil {
		log.Fatal(err)
	}
	network = N
	/* Wait a certain number of ms for each node to be ready. */
	ms := time.Duration(network.Len() * msPerNode)
	time.Sleep(ms * time.Millisecond)
	/* Return teardown method. */
	return func() {
		if err := network.Close(); err != nil {
			log.Fatal(err)
		}
	}
}

func TestMain(m *testing.M) {
	teardown := setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func TestProposer(t *testing.T) {

	P, _, _ := network.Members()

	/* Do proposals. */
	proposals := 8
	for N := 0; N < proposals; N++ {
		/* Random proposer. */
		proposer := P[rand.Int()%len(P)]
		/* Initiate proposal. */
		url := util.HttpUrl(proposer.server.Addr, "propose", N+1)
		if resp, err := http.Post(url, contentTypeBytes, emptyBody); err != nil {
			failTest(t, err)
		} else if resp.StatusCode != http.StatusCreated {
			failTest(t, errWrongStatusCode,
				resp.Status, http.StatusText(http.StatusCreated))
		} else {
			resp.Body.Close()
		}
	}
	/* Assert consensus. */
	if _, v, err := network.Consensus(); err != nil {
		failTest(t, err)
	} else if value, err := strconv.Atoi(v); err != nil {
		failTest(t, err)
	} else if value != proposals {
		assert.Equal(t, proposals, value)
	}
}

func TestProposerConcurrent(t *testing.T) {

	/* Async method. */
	propose := func(p *Node, v int, c chan error) {
		/* Initiate proposal. */
		url := util.HttpUrl(p.server.Addr, "propose", v+1)
		if resp, err := http.Post(url, contentTypeBytes, emptyBody); err != nil {
			c <- err
		} else if resp.StatusCode != http.StatusCreated {
			c <- util.ErrorFormat(errWrongStatusCode, resp.Status, http.StatusText(http.StatusCreated))
		} else {
			resp.Body.Close()
			c <- nil
		}
	}

	P, _, _ := network.Members()

	/* Do proposals. */
	proposals := 8
	fan := make(chan error, proposals)
	/* Concurrent fan-out. */
	for N := 0; N < proposals; N++ {
		/* Random proposer. */
		proposer := P[rand.Int()%len(P)]
		go propose(proposer, N, fan)
	}
	/* Fan-in. */
	for N := 0; N < proposals; N++ {
		if err := <-fan; err != nil {
			t.Error(err)
		}
	}
	/* Assert consensus. */
	if _, v, err := network.Consensus(); err != nil {
		failTest(t, err)
	} else if value, err := strconv.Atoi(v); err != nil {
		failTest(t, err)
	} else if value != proposals {
		assert.Equal(t, proposals, value)
	}
}

func failTest(t *testing.T, err error, args ...interface{}) {
	err = util.ErrorFormat(err, args...)
	t.Error(err)
	t.FailNow()
}
