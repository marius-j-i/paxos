package paxos

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

var (
	/* Errors. */
	errWrongNodeType = errors.New("wrong node type [%s] for url [%s]")

	/* Json body keywords. */
	JsonKeyAccepted  = `accepted`
	JsonKeyProposal  = `proposal`
	JsonKeyPrepare   = `prepare`
	JsonKeyAccepters = `accepters`
	JsonKeyLearners  = `learners`

	/* Empty buffer to signal alive. */
	alive = []byte{}
)

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
		return
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
		return
	}

	body := map[string]interface{}{
		JsonKeyAccepters: a,
	}
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(&body); err != nil {
		msg := fmt.Sprintf("unable to encode response to [%s]: \n%s", req.URL, err.Error())
		n.respondError(w, http.StatusInternalServerError, msg)
		return
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
		return
	}

	body := map[string]interface{}{
		JsonKeyLearners: l,
	}
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(&body); err != nil {
		msg := fmt.Sprintf("unable to encode response to [%s]: \n%s", req.URL, err.Error())
		n.respondError(w, http.StatusInternalServerError, msg)
		return
	}
}

/* /learners
 * Role - Any
 */

func (n *Node) GetAlive(w http.ResponseWriter, req *http.Request) {
	w.Write(alive)
}
