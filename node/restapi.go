package paxos

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/marius-j-i/paxos/util"
)

var (
	/* Errors. */
	errNoValue       = errors.New("url [%s] has no value")
	errWrongNodeType = errors.New("wrong node type [%s] for url [%s]")

	/* Json body keywords. */
	JsonKeyAccepted  = `accepted`
	JsonKeyProposal  = `proposal`
	JsonKeyPrepare   = `prepare`
	JsonKeyAccepters = `accepters`
	JsonKeyLearners  = `learners`

	/* Empty buffer to signal alive. */
	alive = []byte{}

	/* Upper limit on re-tries for failed proposals. */
	maxProposals = 8

	/* Timeout. */
	proposalTimeoutUnit  = time.Millisecond
	proposalTimeoutLower = 200
	proposalTimeoutUpper = 500
)

/* Respond with http-error and appropriate status-text with appended text. */
func (n *Node) respondError(w http.ResponseWriter, status int, extra ...string) {
	format := "\n"
	for i := range extra {
		format += extra[i] + "\n"
	}
	http.Error(w, http.StatusText(status)+format, status)
}

/* /propose
 * Role - Proposer
 */

func (n *Node) PostPropose(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	if n.role != Proposer {
		err := util.ErrorFormat(errWrongNodeType, "proposer", req.URL)
		n.respondError(w, http.StatusBadRequest, err.Error())
	}

	v, ok := mux.Vars(req)[varValue]
	if !ok {
		err := util.ErrorFormat(errNoValue, req.URL)
		n.respondError(w, http.StatusBadRequest, err.Error())
	}
	N := n.N + 1
	n.prepare = N

	for try := 0; try < maxProposals; try++ {
		if /* Prepare-phase. */ NPrime, vPrime, err := n.Prepare(N, v); err != nil {
			n.respondError(w, http.StatusInternalServerError, err.Error())
			break
		} else if NPrime >= N {
			/* v' is either same value or new accepted value.
			 * Update self, and continue proposing with new N and same request-value. */
			if err = n.Commit(NPrime, vPrime); err != nil {
				n.respondError(w, http.StatusInternalServerError, err.Error())
				break
			}
			n.timeout(proposalTimeoutLower, proposalTimeoutUpper, proposalTimeoutUnit)
			continue
		}
		if /* Accept-phase. */ NPrime, vPrime, err := n.Accept(N, v); err != nil {
			n.respondError(w, http.StatusInternalServerError, err.Error())
		} else /* Commit proposal. */ if err := n.Commit(NPrime, vPrime); err != nil {
			n.respondError(w, http.StatusInternalServerError, err.Error())
		} else if NPrime >= N {
			/* Quorum of acceptors may or may not be updated.
			 * Correctness of Paxos should ensure something here. */
			fmt.Fprintf(w, "NPrime >= N -> %d >= %d", NPrime, N)
		}
		break
	}
	w.WriteHeader(http.StatusCreated)
}

/* /prepare
 * Role - Accepter
 */

func (n *Node) PostPrepare(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	if n.role != Accepter {
		err := util.ErrorFormat(errWrongNodeType, "accepter", req.URL)
		n.respondError(w, http.StatusBadRequest, err.Error())
	}
	n.respondError(w, http.StatusInternalServerError, util.ErrImplMe.Error())
}

/* /accept
 * Role - Accepter
 */

func (n *Node) PostAccept(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	if n.role != Accepter {
		msg := util.ErrorFormat(errWrongNodeType, "accepter", req.URL).Error()
		n.respondError(w, http.StatusBadRequest, msg)
	}
	n.respondError(w, http.StatusInternalServerError, util.ErrImplMe.Error())
}

/* /accepted
 * Role - Any
 */

func (n *Node) GetAccepted(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	body := map[string]interface{}{
		JsonKeyAccepted: n.value,
		JsonKeyProposal: n.N,
		JsonKeyPrepare:  n.prepare,
	}
	/* Write before 200 OK is set on write. */
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(&body); err != nil {
		msg := fmt.Sprintf("unable to encode response to [%s]: \n%s", req.URL, err.Error())
		n.respondError(w, http.StatusInternalServerError, msg)
	}
}

/* /accepters
 * Role - Any
 */

func (n *Node) GetAccepters(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	a, err := n.checkAlive(Accepter)
	if err != nil {
		n.respondError(w, http.StatusInternalServerError, err.Error())
	}

	body := map[string]interface{}{
		JsonKeyAccepters: a,
	}
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(&body); err != nil {
		msg := fmt.Sprintf("unable to encode response to [%s]: \n%s", req.URL, err.Error())
		n.respondError(w, http.StatusInternalServerError, msg)
	}
}

/* /learners
 * Role - Any
 */

func (n *Node) GetLearners(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	l, err := n.checkAlive(Learner)
	if err != nil {
		n.respondError(w, http.StatusInternalServerError, err.Error())
	}

	body := map[string]interface{}{
		JsonKeyLearners: l,
	}
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(&body); err != nil {
		msg := fmt.Sprintf("unable to encode response to [%s]: \n%s", req.URL, err.Error())
		n.respondError(w, http.StatusInternalServerError, msg)
	}
}

/* /learners
 * Role - Any
 */

func (n *Node) GetAlive(w http.ResponseWriter, req *http.Request) {
	w.Write(alive)
}
