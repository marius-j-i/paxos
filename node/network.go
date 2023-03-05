package paxos

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/marius-j-i/paxos/util"
)

var (
	/* Map-keys for mux regex parsing. */
	regexNumeric         = "[0-9]+"
	varValue, regexValue = "value", "[a-zA-Z0-9]+"
	varN, regexN         = "N", regexNumeric

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

/* Respond with http-error and appropriate status-text with appended text. */
func (n *Node) respondError(w http.ResponseWriter, status int, extra ...string) {
	format := "\n"
	for i := range extra {
		format += extra[i] + "\n"
	}
	http.Error(w, http.StatusText(status)+format, status)
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
		url := util.HttpUrl(addr, GetAlive)
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
