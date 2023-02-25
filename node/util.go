package paxos

import (
	"math/rand"
	"time"
)

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
