package paxos

import (
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/marius-j-i/paxos/util"
)

/* Response to /prepare from accepters.
 */
type Promise struct {
	from    string // url to response member
	N       int    // accepted proposal number
	prepare int    // promised proposal number
	value   string // accepted proposal value
	err     error  // non-nil if unsuccessful POST
}

/* Return a new promise instance.
 */
func newPromise() *Promise {
	return &Promise{
		from:    ``,
		N:       0,
		prepare: 0,
		value:   ``,
		err:     nil,
	}
}

/* Set promise members with nodes' members.
 */
func (p *Promise) setNode(n *Node) *Promise {
	p.from = n.server.Addr
	p.N = n.N
	p.prepare = n.prepare
	p.value = n.value
	p.err = nil
	return p
}

/* Return variable with mux regex name in url as string.
 */
func (n *Node) getVarString(req *http.Request, name string) (string, error) {

	if s, ok := mux.Vars(req)[name]; !ok {
		err := util.ErrorFormat(errNoValue, req.URL, name)
		return "", err

	} else {
		return s, nil
	}
}

/* Return variable with mux regex name in url converted to int from string.
 */
func (n *Node) getVarInt(req *http.Request, name string) (int, error) {

	if s, ok := mux.Vars(req)[name]; !ok {
		err := util.ErrorFormat(errNoValue, req.URL, name)
		return 0, err

	} else if v, err := strconv.Atoi(s); err != nil {
		return 0, err

	} else {
		return v, nil
	}
}

/* Select a random timeout from interval to wait for; then return.
 */
func (n *Node) timeout(lower, upper int, unit time.Duration) {

	t := rand.Intn(upper-lower) + lower
	d := time.Duration(t) * unit

	wait := time.NewTimer(d)
	<-wait.C
}

/* Return string description of nodes' role.
 */
func (n *Node) Role() string {
	roles := map[Role]string{
		Proposer: "proposer",
		Accepter: "accepter",
		Learner:  "learner",
	}
	return roles[n.role]
}

/* Return number of members with arguemtn role in network.
 */
func (n *Node) LenRoles(r Role) (members int) {
	for _, role := range n.network {
		if role != r {
			continue
		}
		members++
	}
	return
}
