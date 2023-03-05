package paxos

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/marius-j-i/paxos/util"
)

var (
	/* Errors. */
	errNoRestore  = errors.New("nodes set to not restore persistent state")
	errTypeAssert = errors.New("unable to type assert keyword {%s : %v} from type [%T] into [%s]")

	/* Globals. */
	persistState           = true  // on true; n.commit() will not persist state
	restorePersistentState = true  // on true; NewNode() will not read any discovered state files
	persistAfterShutdown   = false // on false; n.Shutdown() removes files from disk
	nodeDir                = "nodes"
)

/* Set n.commit() to not write state to disk.
 */
func SetPersistState(trueOrFalse bool) {
	persistState = trueOrFalse
}

/* Set NewNode() behaviour to avoid restoring persistent state.
 */
func SetRestorePersistentState(trueOrFalse bool) {
	restorePersistentState = trueOrFalse
}

/* Set future n.Shutdown() behaviour related to cleanup of persistent state.
 */
func SetPersistAfterShutdown(trueOrFalse bool) {
	persistAfterShutdown = trueOrFalse
}

/* Set directory for nodes to persist state.
 */
func SetNodeDirectory(path string) {
	nodeDir = path
}

/* Open value file for node, and make an initial commit.
 * If file already exists, restore state from contents if possible.
 */
func (n *Node) createNodeFile(addr string) error {

	if !persistState {
		return nil
	}
	/* Directory for node. */
	if err := os.MkdirAll(nodeDir, os.ModePerm); err != nil {
		return err
	}
	/* Path to file. */
	file := fmt.Sprintf("%s-%s", n.Role(), addr)
	path := path.Join(nodeDir, file)
	/* Restore from previous state, if any. */
	if err := n.restore(path); err != nil {
		/* else; Create node file. */
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		n.f = f
		/* Commit initial state. */
		if err := n.commit(n.N, n.value); err != nil {
			return err
		}
	}
	return nil
}

/* Persist node and update struct-members to new values.
 */
func (n *Node) commit(N int, v string) error {
	n.N, n.value = N, v

	if !persistState {
		return nil
	}
	/* State to persist. */
	node := map[string]interface{}{
		"role":    n.role,
		"addr":    n.server.Addr,
		"N":       n.N,
		"prepare": n.prepare,
		"value":   n.value,
		"quorum":  n.quorum,
		"network": n.network,
	}
	/* Set file to overwrite previous state. */
	if _, err := n.f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	/* Persist to disk. */
	if err := json.NewEncoder(n.f).Encode(&node); err != nil {
		return err
	}
	return nil
}

/* Restore node state from file.
 */
func (n *Node) restore(path string) error {
	var node map[string]interface{}

	if !restorePersistentState {
		return errNoRestore
	}
	/* Open to read and write. */
	f, err := os.OpenFile(path, os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	n.f = f
	/* Restore from n.commit. */
	if err := json.NewDecoder(n.f).Decode(&node); err != nil {
		return err
	}
	/* Type assert and set state. */
	kwN, kwPrepare, kwValue := "N", "prepare", "value"
	if N, ok := node[kwN].(float64); !ok {
		return util.ErrorFormat(errTypeAssert,
			kwN, node[kwN], node[kwN], "float64")
	} else if prepare, ok := node[kwPrepare].(float64); !ok {
		return util.ErrorFormat(errTypeAssert,
			kwPrepare, node[kwPrepare], node[kwPrepare], "string")
	} else if value, ok := node[kwValue].(string); !ok {
		return util.ErrorFormat(errTypeAssert,
			kwValue, node[kwValue], node[kwValue], "string")
	} else {
		/* Convert to correct types. */
		n.N, n.prepare, n.value = int(N), int(prepare), value
	}
	return nil
}

/* Remove persistent state as part of shutdown,
 * unless n.SetPersistAfterShutdown(true) has been invoked.
 */
func (n *Node) cleanupNodeFile() error {

	if !persistState {
		return nil
	}
	path := n.f.Name()
	if err := n.f.Close(); err != nil {
		return err
	}
	/* To keep... */
	if persistAfterShutdown {
		goto done
	}
	/* ... or not to keep. */
	if err := os.Remove(path); err != nil {
		return err
	}

done:
	return nil
}
