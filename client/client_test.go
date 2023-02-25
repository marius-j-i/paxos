package client

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	paxos "github.com/marius-j-i/paxos/node"
	"github.com/marius-j-i/paxos/util"
	log "github.com/sirupsen/logrus"
)

var (
	/* Errors. */
	errProposalNotAccepted = errors.New("proposal [#%d] was proposed, but proposal [#%d] was accepted after %v")
	errValueNotAccepted    = errors.New("value [%s] was proposed, but value [%s] was accepted after %v")

	/* Setup configurations. */
	proposers   = 2
	accepters   = 5
	learners    = 4
	host        = `localhost`
	port, other = `8080`, `8081`
	startport   = 8080

	/* Proposer values. */
	value, valueTwo = `value-one`, fmt.Sprintf("%s-%s", value, `two`)
	acceptWindow    = 200 * time.Millisecond
)

func setup() func() {

	/* Size of error channels' buffers. */
	nErr := 16

	nodes, errchan := setupNodes(nErr)

	return makeTeardown(nodes, errchan)
}

func setupNodes(nErr int) ([]*paxos.Node, []chan error) {

	/* Create ports and network. */
	ports := make([]string, proposers+accepters+learners)
	roles := make([]paxos.Role, len(ports))
	network := make(map[string]paxos.Role, len(ports))
	for i := range ports {
		ports[i] = strconv.Itoa(startport + i)

		role := paxos.Proposer
		if i < proposers+accepters {
			role = paxos.Accepter
		} else {
			role = paxos.Learner
		}
		roles[i] = role

		addr := net.JoinHostPort(host, ports[i])
		network[addr] = roles[i]
	}

	/* Create nodes with different roles. */
	nodes := make([]*paxos.Node, len(ports))
	errchan := make([]chan error, len(ports))
	for i := range ports {

		log.Infof("role: , ")
		/* New node with role. */
		n, err := paxos.NewNode(roles[i], host, ports[i], network)
		if err != nil {
			log.Fatal(err.Error())
		}
		nodes[i] = n

		/* Start new node. */
		errchan[i] = make(chan error, nErr)
		go nodes[i].Serve(errchan[i])
	}
	return nodes, errchan
}

func makeTeardown(nodes []*paxos.Node, errchan []chan error) func() {

	/* Log-file for server errors during tests. */
	logname := `nodes.log`
	logfile, err := os.Create(logname)
	if err != nil {
		log.Fatalf("unable to create file [%s] \n", logname)
	}

	/* Routine for emptying potential errors. */
	stop := make(chan bool)
	go func() {
		for len(stop) != 0 {
			for i := range nodes {
				if len(errchan[i]) != 0 {
					fmt.Fprintf(logfile, "Error-channel[%d]-serve: %s \n", i, (<-errchan[i]).Error())
				}
			}
		}
		/* Stop signal received from teardown, acknowledge. */
		<-stop
	}()

	/* Teardown definition. */
	teardown := func() {
		/* Send and wait for stop. */
		stop <- true
		for i := range nodes {
			go nodes[i].Shutdown(errchan[i])
		}
		for i := 0; i < len(nodes); {
			/* Empty errors before incrementing. */
			if err := <-errchan[i]; err != nil {
				fmt.Fprintf(logfile, "Error-channel[%d]-shutdown: %s \n", i, err.Error())
				continue
			}
			i++
		}
		logfile.Close()
	}
	return teardown
}

func TestMain(m *testing.M) {
	teardown := setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func TestPropose(t *testing.T) {

	/* Essentially check if server can respond with 200 OK. */
	if err := Propose(host, port, value); err != nil {
		t.Error(err)
		t.FailNow()
	}
}

func TestGetAccepted(t *testing.T) {

	if _, _, err := GetAccepted(host, port); err != nil {
		t.Error(err)
	} /* Can not make assumptions about value or proposal. */
}

func TestProposeThenGetAccepted(t *testing.T) {

	/* Asynchronously propose value,... */
	errchan := make(chan error, 1)
	go func() {
		if err := Propose(host, port, value); err != nil {
			errchan <- err
		}
		errchan <- nil
	}()

	/* ... wait for acceptance, ... */
	timer := time.NewTimer(acceptWindow)
	select {
	case err := <-errchan: /* Accepted. */
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
	case <-timer.C: /* Timeout. */
	}

	/* ... then fetch and assert accepted. */
	if accepted, _, err := GetAccepted(host, port); err != nil {
		t.Error(err)

	} else if accepted != value {
		err = util.ErrorFormat(errValueNotAccepted, value, accepted, acceptWindow)
		t.Error(err)
	}
}

func TestGetAcceptedThenProposeThenGetAccepted(t *testing.T) {

	/* Get existing accepted proposal number. */
	_, n1, err := GetAccepted(host, port)
	if err != nil {
		t.Error(err.Error())
		t.FailNow()
	}

	/* Asynchronously propose value,... */
	errchan := make(chan error, 1)
	go func() {
		if err := Propose(host, port, value); err != nil {
			errchan <- err
		}
		errchan <- nil
	}()

	/* ... wait for acceptance, ... */
	timer := time.NewTimer(acceptWindow)
	select {
	case err := <-errchan: /* Accepted. */
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
	case <-timer.C: /* Timeout. */
	}

	/* ... then fetch and assert accepted. */
	if _, n2, err := GetAccepted(host, port); err != nil {
		t.Error(err)

	} else if n1 != n2-1 {
		err = util.ErrorFormat(errProposalNotAccepted, n2, n1, acceptWindow)
		t.Errorf(err.Error())

	}
}

func TestGetAccepters(t *testing.T) {

	a, err := GetAccepters(host, port)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	/* Assert addresses are resolvable. */
	for i := range a {
		if _, err := net.LookupHost(a[i]); err != nil {
			t.Error(err)
			t.FailNow()
		}
	}
}

func TestGetLearners(t *testing.T) {

	l, err := GetLearners(host, port)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	/* Assert addresses are resolvable. */
	for i := range l {
		if _, err := net.LookupHost(l[i]); err != nil {
			t.Error(err)
			t.FailNow()
		}
	}
}

func TestTwoPropose(t *testing.T) {
	t.Skip()

	go func() {
		if err := Propose(host, port, value); err != nil {
			t.Error(err)
		}
	}()

	go func() {
		if err := Propose(host, other, valueTwo); err != nil {
			t.Error(err)
		}
	}()

	wait := time.NewTimer(2 * acceptWindow)
	<-wait.C

	if t.Failed() {
		t.FailNow()
	}

	if v1, n1, err := GetAccepted(host, port); err != nil {
		t.Error(err)
	} else if v2, n2, err := GetAccepted(host, other); err != nil {
		t.Error(err)
	} else if n1 >= n2 {
		t.Errorf(`n1<n2 -> !true -> %d<%d: 1st proposal number should be less than 2nd`, n1, n2)
	} else if v1 != valueTwo {
		t.Errorf(`v1==valueTwo -> !true -> %s==%s: 1st proposer value should be 2nd value after 2nd proposal`, v1, valueTwo)
	} else if v2 != valueTwo {
		t.Errorf(`v2==valueTwo -> !true -> %s==%s: 2nd proposer value should be 2nd value after 2nd proposal`, v2, valueTwo)
	}
}

func BenchmarkTxPerS(b *testing.B) {
	b.Skip()

	var proposal string
	p := func(b *testing.B) {
		if err := Propose(host, port, proposal); err != nil {
			b.Error(err)
			b.Fail()
		}
	}

	for n := 0; n < b.N; n++ {
		proposal = fmt.Sprintf("Proposal-[%d]", n+1)

		if failed := b.Run(proposal, p); failed || b.Failed() {
			b.FailNow()
		}
	}
	t := b.Elapsed()

	b.Logf(`tx/s := %f, N := %d, time := %v`, float64(t)/float64(b.N), b.N, t)
}

// func BenchmarkParallelTxPerS(b *testing.B) {
// 	b.RunParallel()
// }
