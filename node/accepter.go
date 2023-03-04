package paxos

import (
	"encoding/json"
	"net/http"

	"github.com/marius-j-i/paxos/util"
	log "github.com/sirupsen/logrus"
)

/* /prepare
 * Role - Accepter
 */

func (n *Node) PostPrepare(w http.ResponseWriter, req *http.Request) {
	var N int
	defer req.Body.Close()

	/* Assert Role. */
	if n.role != Accepter && n.role != Learner {
		err := util.ErrorFormat(errWrongNodeType, "accepter|learner", req.URL)
		n.respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	/* Get proposal from url. */
	N, err := n.getVarInt(req, varN)
	if err != nil {
		n.respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	/* No promise to a higher proposal. */
	if N < n.prepare {
		n.prepare = N
	} /* else; create promise with N' > N. */
	p := newPromise().setNode(n)
	/* Respond with appropriate promise. */
	if err := json.NewEncoder(w).Encode(&p); err != nil {
		n.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

/* /accept
 * Role - Accepter or Learner
 */

func (n *Node) PostAccept(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	/* Assert Role. */
	if n.role != Accepter && n.role != Learner {
		msg := util.ErrorFormat(errWrongNodeType, "accepter|learner", req.URL).Error()
		n.respondError(w, http.StatusBadRequest, msg)
		return
	}
	/* Get proposal and value from url. */
	N, err := n.getVarInt(req, varN)
	if err != nil {
		n.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	v, err := n.getVarString(req, varValue)
	if err != nil {
		n.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	/* Reject accept proposal. */
	if N != n.prepare {
		log.Infof("reject proposal N [%d] in favor of prepare proposal N' [%d] ",
			N, n.prepare)

	} else /* Accept proposal. */ if err := n.Commit(N, v); err != nil {
		n.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	p := newPromise().setNode(n)
	/* Respond with appropriate promise. */
	if err := json.NewEncoder(w).Encode(&p); err != nil {
		n.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}
