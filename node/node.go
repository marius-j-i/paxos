package paxos

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/marius-j-i/paxos/util"
)

var (
	/* Errors. */
	errNoQuorum = errors.New("cannot achieve quorum with [%d] accepters")

	/* Timeouts. */
	SHUTDOWNTIMEOUT = 8 * time.Second
)

/* Valid roles for nodes. */
const (
	Proposer Role = iota + 1
	Accepter
	Learner
)

/* Indicator of node role. */
type Role int

type Node struct {
	role    Role                  // node role; proposer, accepter, or learner
	quorum  int                   // number of accepters needed move from prepare phase
	prepare int                   // most recent prepare-phase promise
	N       int                   // number of currently accepted value
	value   string                // currently accepted value
	f       *os.File              // file to persist current state
	routes  map[string]*mux.Route // url-path mapping to route instance
	network map[string]Role       // address mapping to role of network member
	server  *http.Server          // server...
	body    io.Reader             // empty body to pass into post requests
}

/* Return new node.
 */
func NewNode(r Role, addr string, network map[string]Role) (*Node, error) {

	n := &Node{
		role:    r,
		quorum:  0,
		network: nil,
		prepare: 0,
		N:       0,
		value:   ``,
		f:       nil,
		routes:  map[string]*mux.Route{},
		server:  &http.Server{Addr: addr},
		body:    bytes.NewReader([]byte{}),
	}

	/* Route end-points to server. */
	if err := n.configureServer(); err != nil {
		return nil, err
	} else if err := n.createNetwork(addr, network); err != nil {
		return nil, err
	} else if err := n.createNodeFile(addr); err != nil {
		return nil, err
	}

	return n, nil
}

/* Copy and exclude self for network.
 * Count accepters and find quorum.
 */
func (n *Node) createNetwork(addr string, network map[string]Role) error {

	members := map[string]Role{}
	accepters := 0
	for member, role := range network {
		if member != addr {
			members[member] = role
		}
		if role == Accepter {
			accepters++
		}
	}
	/* Assert network can find quorum. */
	if accepters%2 != 1 {
		return util.ErrorFormat(errNoQuorum, accepters)
	}
	n.network, n.quorum = members, (accepters/2)+1

	return nil
}

/* Serve requests indefinitively until Node.Shutdown is called.
 * Put any non-server-closed errors in argument channel.
 */
func (n *Node) Serve(errchan chan error) {

	if err := n.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		errchan <- err
	}

	/* Signal end of routine. */
	errchan <- nil
}

/* Close value-file.
 * Stop server.
 */
func (n *Node) Shutdown(errchan chan error) {

	/* Create context interface for server to use when shutting down. */
	ctx, cancel := context.WithTimeout(context.Background(), SHUTDOWNTIMEOUT)
	defer cancel()

	/* Shutdown gracefully within timeframe. */
	if err := n.server.Shutdown(ctx); err != nil {
		errchan <- err
	}

	if err := n.cleanupNodeFile(); err != nil {
		errchan <- err
	}

	/* Signal end of routine. */
	errchan <- nil
}
