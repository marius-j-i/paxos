package paxos

import (
	"errors"
	"fmt"
	"net"

	"github.com/marius-j-i/paxos/util"
	log "github.com/sirupsen/logrus"
)

var (
	/* Errors. */
	errBrokenSafetyPropertySingleValue = errors.New("paxos safety property broken: `Only a single value is chosen, ...`")
	errNoConsensus                     = errors.New("no consensus found for proposal, value := [%d, %s] with quorum := [%d/%d]")
)

/* Object for in-house handling of instansiated nodes.
 */
type Network struct {
	nodes   []*Node
	errchan chan error
}

/* Start a network of paxos nodes and return closer to shutdown nodes.
 */
func NewNetwork(proposers, accepters, learners int) (*Network, error) {

	roles, addrs, network := createNetwork(proposers, accepters, learners)

	nodes, errchan, err := startNodes(roles, addrs, network)
	if err != nil {
		return nil, err
	}

	closer := &Network{
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
		} else if i < proposers+accepters {
			roles[i] = Accepter
		} else if i < proposers+accepters+learners {
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

/* Return number of pnodes in network.
 */
func (N *Network) Len() int {
	return len(N.nodes)
}

/* Return proposers, accepters, and learners in network.
 */
func (N *Network) Members() (P []*Node, A []*Node, L []*Node) {

	for _, n := range N.nodes {
		switch n.role {
		case Proposer:
			P = append(P, n)
		case Accepter:
			A = append(A, n)
		case Learner:
			L = append(L, n)
		}
	}
	return P, A, L
}

/* Closer implementation of Network.
 */
func (N *Network) Close() error {
	N.DestroyNetwork(N.nodes, N.errchan)
	monitorNodes(N.errchan)
	return nil
}

/* Return proposal p and value v which network has consensus on.
 * Return (-1, "") if no consensus can be made.
 */
func (N *Network) Consensus() (int, string, error) {
	p, v := -1, ""

	/* Find greatest proposal p. */
	for _, n := range N.nodes {
		if n.N > p {
			p, v = n.N, n.value
		}
	}
	/* Find values v with proposal p and assert they agree on value v.
	 * Count quorum for proposal p and value v. */
	quorum := 0
	for _, n := range N.nodes {
		/* Updated accepters count for quorum. */
		if n.role != Accepter || n.N != p {
			continue
		}
		if /* Broken safety property. */ n.value != v {
			return -1, "", errBrokenSafetyPropertySingleValue
		}
		quorum++
	}
	if /* No quorum. */ quorum < N.nodes[0].quorum {
		err := util.ErrorFormat(errNoConsensus,
			p, v, quorum, N.nodes[0].quorum)
		return -1, "", err
	}
	return p, v, nil
}

/* Invoke shutdown for argument nodes.
 */
func (N *Network) DestroyNetwork(nodes []*Node, errchan chan error) {
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
