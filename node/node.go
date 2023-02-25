package paxos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
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
func NewNode(r Role, host, port string, network map[string]Role) (*Node, error) {

	/* Address for this node. */
	addr := net.JoinHostPort(host, port)

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
	} else if err := n.createNodeFile(addr); err != nil {
		return nil, err
	} else if err := n.createNetwork(addr, network); err != nil {
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

/* Open or overwrite value file for node, and make an initial commit.
 */
func (n *Node) createNodeFile(addr string) error {

	dir := "values"
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	file := fmt.Sprintf("%s-%s", n.Role(), addr)
	path := path.Join(dir, file)

	f, err := os.Create(path)
	if err != nil {
		return err
	} else if err := n.Commit(n.N, n.value); err != nil {
		return err
	}
	n.f = f

	return nil
}

/* Persist node and update struct-members to new values.
 */
func (n *Node) Commit(N int, v string) error {
	format, newline := "", "\n"
	format += "role : %s, addr : %s {" + newline
	format += "    N       : %d," + newline
	format += "    value   : %s," + newline
	format += "    quorum  : %d," + newline
	format += "    network : %v," + newline
	format += "}" + newline

	n.N, n.value = N, v
	node := fmt.Sprintf(format,
		n.Role(), n.server.Addr,
		n.N,
		n.value,
		n.quorum,
		n.network,
	)

	if _, err := n.f.WriteString(node); err != nil {
		return err
	}
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

	if err := n.f.Close(); err != nil {
		errchan <- err
	}

	/* Signal end of routine. */
	errchan <- nil
}