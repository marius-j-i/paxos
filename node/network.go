package paxos

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/marius-j-i/paxos/util"
)

var (
	/* Errors. */
	errInvalidOperation = errors.New("node[%s] with role[%s] attempted to initiate operation[%s]")

	/* Map-keys for mux regex parsing. */
	regexNumeric         = "[0-9]+"
	varValue, regexValue = "value", "[a-zA-Z0-9]+"
	varN, regexN         = "N", regexNumeric

	regexPrepare = regexNumeric

	/* API end-points. */
	PostPropose  = fmt.Sprintf("/propose/{%s:%s}", varValue, regexValue)
	PostPrepare  = fmt.Sprintf("/prepare/{%s:%s}", varN, regexN)
	PostAccept   = fmt.Sprintf("/accept/{%s:%s}/{%s:%s}", varN, regexN, varValue, regexValue)
	GetAccepted  = "/accepted"
	GetAccepters = "/accepters"
	GetLearners  = "/learners"
	GetAlive     = "/alive"

	/* HTTP. */
	GET              = `GET`
	POST             = `POST`
	contentTypeBytes = "application/octet-stream"
)

/* Response to /prepare from accepters.
 */
type Promise struct {
	N       int    // accepted proposal number
	prepare int    // promised proposal number
	value   string // accepted proposal value
	err     error  // non-nil if unsuccessful POST
}

/* Configure server paths to HTTP API.
 */
func (n *Node) configureServer() error {

	/* REST API handler.
	 */
	router := mux.NewRouter()

	/* Map REST API end-points to node handle methods. */
	n.routes[PostPropose] = router.HandleFunc(PostPropose, n.PostPropose).Methods(POST)
	n.routes[PostPrepare] = router.HandleFunc(PostPrepare, n.PostPrepare).Methods(POST)
	n.routes[PostAccept] = router.HandleFunc(PostAccept, n.PostAccept).Methods(POST)
	n.routes[GetAccepted] = router.HandleFunc(GetAccepted, n.GetAccepted).Methods(GET)
	n.routes[GetAccepters] = router.HandleFunc(GetAccepters, n.GetAccepters).Methods(GET)
	n.routes[GetLearners] = router.HandleFunc(GetLearners, n.GetLearners).Methods(GET)
	n.routes[GetAlive] = router.HandleFunc(GetAlive, n.GetAlive).Methods(GET)

	/* Set as handler for both API's. */
	n.server.Handler = router

	return nil
}

/* Proposer attempts to achieve quorum of promises from accepters.
 * Return argument (N-1, v) if quorum reached, or
 *
 * return (N' > N, v') where value-prime is same as argument
 * if any acceptor promised to a higher proposal number than N, or
 *
 * return (N' > N, v') where value-prime is a new accepted value
 * if any acceptor accepted a higher proposal number than N.
 */
func (n *Node) Prepare(N int, v string) (int, string, error) {
	formatPrepareUrl := "%s/prepare/%d/%s"

	if n.role != Proposer {
		err := util.ErrorFormat(errInvalidOperation, n.server.Addr, n.Role(), "propose")
		return N, v, err
	}

	/* Fan-out method. */
	prepare := n.promiseFanOut

	/* Fan-out. */
	promise := make(chan *Promise, len(n.network))
	for addr, role := range n.network {
		if role != Accepter {
			continue
		}
		url := fmt.Sprintf(formatPrepareUrl, addr, N, v)
		go prepare(url, promise)
	}

	/* Fan-in. */
	N, v = n.promiseFanIn(N, v, promise, true)

	/* Prepare-phase complete. */
	return N, v, nil
}

/* Proposer attempts to commit proposal to accepters.
 */
func (n *Node) Accept(N int, v string) (int, string, error) {
	formatAcceptUrl := "%s/accept/%d/%s"

	if n.role != Proposer {
		err := util.ErrorFormat(errInvalidOperation, n.server.Addr, n.Role(), "accept")
		return N, v, err
	}

	/* Fan-out method. */
	accept := n.promiseFanOut

	/* Fan-out. */
	promise := make(chan *Promise, len(n.network))
	for addr, role := range n.network {
		if role != Accepter {
			continue
		}
		url := fmt.Sprintf(formatAcceptUrl, addr, N, v)
		go accept(url, promise)
	}

	/* Fan-in. */
	N, v = n.promiseFanIn(N, v, promise, false)

	/* Prepare-phase complete. */
	return N, v, nil
}

/* Fan-out method for prepare, accept, etc.
 */
func (n *Node) promiseFanOut(url string, promise chan *Promise) {
	p := &Promise{}

	if resp, err := http.Post(url, contentTypeBytes, n.body); err != nil {
		p.err = err

	} else {
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(&promise); err != nil {
			p.err = err
		}
	}
	promise <- p
}

/* Fan-in method for prepare, accept, etc.
 * Return (N-1, v) if no accepters had promised or accepted a higher proposal,
 * or
 * return (p.prepare, v) if any accepters had promised to a higher proposal,
 * or
 * return (p.N, p.value) if any accepters had accepted a higher proposal.
 */
func (n *Node) promiseFanIn(N int, v string, promise chan *Promise, breakOnQuorum bool) (int, string) {

	quorum := 0
	for _, role := range n.network {
		if role != Accepter {
			continue
		}
		p := <-promise
		if p.err != nil {
			continue
		} else if p.prepare >= N {
			/* Promised to a higher proposal. */
			return p.prepare, v

		} else if p.N >= N {
			/* Accepted a higher proposal. */
			return p.N, p.value

		} else /* (p.prepare, p.N) < N, ergo acceptor promise to this proposal. */ {
			quorum++
		}

		/* breakOnQuorum is true for prepare and false for accept. */
		if breakOnQuorum && quorum >= n.quorum {
			break
		}
	}
	/* Accept phase complete - proposal accepted. */
	return N - 1, v
}

/* Proposer attempts to update learners with accepted proposal number and value.
 */
func (n *Node) Learn(N int, v string) (int, string, error) {

	if n.role != Proposer {
		err := util.ErrorFormat(errInvalidOperation, n.server.Addr, n.Role(), "propose")
		return N, v, err
	}
	return N - 1, v, util.ErrImplMe
}

/* Return slice of member-addresses with argument role that responded alive.
 */
func (n *Node) checkAlive(r Role) ([]string, error) {
	a := []string{}

	ping := func(url string, alive chan bool) {
		if resp, err := http.Get(url); err != nil {
			alive <- false
		} else {
			alive <- true
			defer resp.Body.Close()
		}
	}

	/* Fan-out. */
	alive := make(chan bool, len(n.network))
	for addr, role := range n.network {
		if role != r {
			continue
		}
		url := addr + GetAlive
		go ping(url, alive)
	}

	/* Fan-in. */
	for addr, role := range n.network {
		if role != r {
			continue
		} else if <-alive {
			a = append(a, addr)
		}
	}

	return a, nil
}
