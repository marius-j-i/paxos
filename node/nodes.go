package paxos

import (
	"fmt"
	"io"
	"net"

	log "github.com/sirupsen/logrus"
)

/* Object for in-house handling of instansiated nodes.
 */
type Nodes struct {
	nodes   []*Node
	errchan chan error
}

/* Closer implementation of Nodes.
 */
func (N *Nodes) Close() error {
	N.DestroyNetwork(N.nodes, N.errchan)
	monitorNodes(N.errchan)
	return nil
}

/* Invoke shutdown for argument nodes.
 */
func (N *Nodes) DestroyNetwork(nodes []*Node, errchan chan error) {
	for _, n := range nodes {
		go n.Shutdown(errchan)
	}
	/* Receive error until nil from each node. */
	for i := 0; i < len(nodes); {
		if err := <-errchan; err != nil {
			log.Error(err)
			continue
		}
		i++
	}
}

/* Pop and log errors in channel.
 */
func monitorNodes(errchan chan error) {
	for len(errchan) > 0 {
		if err := <-errchan; err != nil {
			log.Error(err)
		}
	}
}

/* Start a network of paxos nodes and return closer to shutdown nodes.
 */
func CreateNetwork(proposers, accepters, learners int) (io.Closer, error) {

	roles, addrs, network := createNetwork(proposers, accepters, learners)

	nodes, errchan, err := startNodes(roles, addrs, network)
	if err != nil {
		return nil, err
	}

	closer := &Nodes{
		nodes:   nodes,
		errchan: errchan,
	}
	return closer, nil
}

/* Return roles, addresses, and network map from arguments.
 */
func createNetwork(proposers, accepters, learners int) ([]Role, []string, map[string]Role) {
	host, startport := "localhost", 9000

	N := proposers + accepters + learners
	roles := make([]Role, N)
	addrs := make([]string, N)
	network := make(map[string]Role, N)
	/* Create arguments for NewNode. */
	for i := 0; i < N; i++ {
		/* Address. */
		port := fmt.Sprint(startport + i)
		addrs[i] = net.JoinHostPort(host, port)
		/* Role. */
		if i < proposers {
			roles[i] = Proposer
		} else if i < proposers+learners {
			roles[i] = Accepter
		} else {
			roles[i] = Learner
		}
		/* Network. */
		network[addrs[i]] = roles[i]
	}
	return roles, addrs, network
}

/* Return created and started nodes, and channel they communicate with.
 */
func startNodes(roles []Role, addrs []string, network map[string]Role) ([]*Node, chan error, error) {

	nodes := make([]*Node, len(network))
	errchan := make(chan error, len(network))
	for i := range nodes {
		if n, err := NewNode(roles[i], addrs[i], network); err != nil {
			return nil, nil, err
		} else {
			nodes[i] = n
		}
		go nodes[i].Serve(errchan)
	}
	return nodes, errchan, nil
}
