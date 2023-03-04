package paxos

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/marius-j-i/paxos/util"
	log "github.com/sirupsen/logrus"
)

var (
	/* Errors, */
	errNoValue = errors.New("url [%s] has no value [%s]")

	/* Timeout. */
	proposalTimeoutUnit  = time.Millisecond
	proposalTimeoutLower = 200
	proposalTimeoutUpper = 500

	/* Upper limit on re-tries for failed proposals. */
	maxProposals = 8
)

/* /propose
 * Role - Proposer
 */

func (n *Node) PostPropose(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	/* Assert Role. */
	if n.role != Proposer {
		err := util.ErrorFormat(errWrongNodeType, "proposer", req.URL)
		n.respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	/* Get proposal value from url. */
	v, err := n.getVarString(req, varValue)
	if err != nil {
		n.respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	/* Propose a limited number of times. */
	for try := 0; try < maxProposals; try++ {

		if code, err := n.postPropose(v); err != nil {
			n.respondError(w, code, err.Error())
			return
		} else if code == http.StatusCreated {
			w.WriteHeader(http.StatusCreated)
			break
		}
	}
}

/* Proposer attempt a proposal.
 * Return HTTP status code CREATED and nil if successful, or
 *
 * return non-CREATED code and nil if a re-try is possible, or
 *
 * return respond code and error on terminating request error.
 */
func (n *Node) postPropose(v string) (int, error) {
	var code, N, NPrime int
	var vPrime string
	var quorum bool

	/* New proposal. */
	N = n.N + 1

	/* Prepare-phase. */
	quorum, NPrime, vPrime = n.Prepare(N, v)
	if !quorum && NPrime >= N {
		/* v' is either nodes' value or new accepted value.
		 * Update self for consensus, then
		 * propose with new proposal N and same request-value v. */
		if err := n.commit(NPrime, vPrime); err != nil {
			code = http.StatusInternalServerError
			return code, err
		}
		/* Random timeout for proposer to complete. */
		n.timeout(proposalTimeoutLower, proposalTimeoutUpper, proposalTimeoutUnit)
		/* Re-try proposal. */
		code = http.StatusContinue
		goto done
	}

	/* Accept-phase. */
	NPrime, _ = n.Accept(N, v)
	if NPrime >= N {
		log.Infof("proposal accept phase with N' >= N: %d >= %d", NPrime, N)
	}

	/* Commit proposal.
	 *
	 * if there is an NPrime >= N;
	 * then a new proposal is in motion,
	 * but this proposal is completed.
	 * Quorum of acceptors may or may not be updated. */
	if err := n.commit(N, v); err != nil {
		code = http.StatusInternalServerError
		return code, err
	}
	code = http.StatusCreated

done:
	return code, nil
}

/* Proposer attempts to achieve quorum of promises from accepters.
 * Return argument (true, N, v) if quorum reached, or
 *
 * return (false, N' > N, v') where value-prime is this nodes' commited value
 * if any acceptor promised to a higher proposal number than N, or
 *
 * return (false, N' > N, v') where value-prime is a new accepted value
 * if any acceptor accepted a higher proposal number than N.
 */
func (n *Node) Prepare(N int, v string) (bool, int, string) {
	promises := make(chan *Promise, len(n.network))

	/* Fan-out. */
	n.prepareFanOut(N, promises)

	/* Fan-in. */
	quorum, N, v := n.prepareFanIn(N, v, promises)

	/* Prepare-phase complete. */
	return quorum, N, v
}

/* Fan-out method for prepare.
 * Proposer concurrently POSTs to accepters to prepare proposal.
 */
func (n *Node) prepareFanOut(N int, promises chan *Promise) {
	formatPrepareUrl := "%s/prepare/%d"

	/* Go routine. */
	prepare := func(url string) {
		p := newPromise()
		/* POST with empty body. */
		resp, err := http.Post(url, contentTypeBytes, n.body)
		if err != nil {
			p.err = err
			goto done
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
			p.err = err
		}
	done:
		promises <- p
	}
	/* Post prepare to accepters. */
	for addr, role := range n.network {
		if role != Accepter {
			continue
		}
		url := fmt.Sprintf(formatPrepareUrl, addr, N)
		go prepare(url)
	}
}

/* Fan-in method for prepare.
 * Proposer gathers at most a quorum of prepare-promises from accepters.
 *
 * Return (true, N, v) if no accepters had promised or accepted a higher proposal,
 * or
 * return (false, p.prepare, n.value) if any accepters had promised to a higher proposal,
 * or
 * return (false, p.N, p.value) if any accepters had accepted a higher proposal.
 */
func (n *Node) prepareFanIn(N int, v string, promises chan *Promise) (bool, int, string) {

	quorum := 0
	for len(promises) > 0 {
		p := <-promises
		if p.err != nil {
			log.Info(p.err)
			continue
		} else /* Promised to a higher proposal. */ if p.prepare >= N {
			return false, p.prepare, n.value

		} else /* Accepted a higher proposal. */ if p.N >= N {
			return false, p.N, p.value

		} else /* (p.prepare, p.N) < N, ergo acceptor promise to this proposal. */ {
			quorum++
		}
		/* End early if proposer attained quorum. */
		if quorum >= n.quorum {
			break
		}
	}
	/* Prepare phase complete? */
	return quorum >= n.quorum, N, v
}

/* Proposer attempts to commit proposal to accepters.
 */
func (n *Node) Accept(N int, v string) (int, string) {
	promises := make(chan *Promise, len(n.network))

	/* Fan-out method. */
	n.acceptFanOut(N, v, promises)

	/* Fan-in. */
	n.acceptFanIn(N, v, promises)

	/* Accept-phase complete - proposal finished. */
	return N, v
}

/* Fan-out method for accept.
 * Proposer concurrently POSTs to both accepters and learners.
 */
func (n *Node) acceptFanOut(N int, v string, promises chan *Promise) {
	formatAcceptUrl := "%s/accept/%d/%s"

	/* Go routine. */
	accept := func(url string) {
		p := newPromise()
		/* POST with empty body. */
		resp, err := http.Post(url, contentTypeBytes, n.body)
		if err != nil {
			p.err = err
			goto done
		}
		defer resp.Body.Close()

		if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
			p.err = err
		}
	done:
		promises <- p
	}
	/* Update accpters and learners. */
	for addr, role := range n.network {
		if role != Accepter && role != Learner {
			continue
		}
		url := fmt.Sprintf(formatAcceptUrl, addr, N, v)
		go accept(url)
	}
}

/* Fan-in method for accept.
 * Proposer gathers accept-promises from accepters and learners.
 * Accepters and learners either commits, rejects, or are non-responsive.
 * Method is essentially non-functional and only evaluates accept phase.
 */
func (n *Node) acceptFanIn(N int, v string, promises chan *Promise) {
	summary := ""

	quorum := 0
	for _, role := range n.network {
		if role != Accepter && role != Learner {
			continue
		}
		p := <-promises
		if p.err != nil {
			log.Info(p.err)
			continue
		} else /* Promised to a higher proposal. */ if p.prepare >= N {
			summary += fmt.Sprintf("accept-promise from [%s] {prepare>=N : %d>=%d, values : [%s, %s]} \n",
				p.from, p.prepare, N, p.value, v)

		} else /* Accepted a higher proposal. */ if p.N >= N {
			summary += fmt.Sprintf("accept-promise from [%s] {   N'>=N   : %d>=%d, values : [%s, %s]} \n",
				p.from, p.N, N, p.value, v)

		} else /* (p.prepare, p.N) < N, ergo acceptor promise to this proposal. */ {
			quorum++
		}
	}
	summary = fmt.Sprintf("accept-quorum (accepters + learners) := %d/%d \n%s",
		quorum, n.LenRoles(Accepter)+n.LenRoles(Learner), summary)

	/* Accept phase complete. */
	log.Info(summary)
}
