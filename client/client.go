package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	paxos "github.com/marius-j-i/paxos/node"
	"github.com/marius-j-i/paxos/util"
)

var (
	/* Errors. */
	errJsonValueType  = errors.New("unable to type assert Json-object `%v` into type `%s`")
	errMissingJsonKey = errors.New("response json-body contained no keyword for `%s`")
	errNotStatusOK    = errors.New("got [%d - %s], but wanted [%d - %s]")

	/* Defines. */
	contentTypeJson = `application/json`
	protocol        = `http://`
	stringType      = `string`
	postPropose     = `/propose/%s`
)

/* Post value to proposer. */
func Propose(host, port, value string) error {

	/* Format POST url. */
	addr := net.JoinHostPort(host, port)
	proposeValue := fmt.Sprintf(postPropose, value)
	url := protocol + addr + proposeValue

	/* POST to proposer. */
	resp, err := http.Post(url, contentTypeJson, strings.NewReader(value))
	if err != nil {
		return err
	}
	/* Body open on success; close when done. */
	defer resp.Body.Close()

	/* Assert OK. */
	if resp.StatusCode != http.StatusCreated {
		return unexpectedStatusCode(resp.StatusCode, http.StatusCreated)
	}

	return nil
}

/* Return acccepted value gotten from proposer. */
func GetAccepted(host, port string) (string, int, error) {

	/* Format GET url. */
	addr := net.JoinHostPort(host, port)
	url := protocol + addr + paxos.GetAccepted

	/* GET accepted value. */
	resp, err := http.Get(url)
	if err != nil {
		return "", -1, err
	}
	/* Body open on success; close when done. */
	defer resp.Body.Close()

	/* Assert OK. */
	if resp.StatusCode != http.StatusOK {
		return "", -1, unexpectedStatusCode(resp.StatusCode, http.StatusOK)
	}

	/* Body responses are json-formatted. */
	var body map[string]interface{}

	/* Decode body into map. */
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", -1, err
	}

	/* Extract accepted value. */
	if v, ok := body[paxos.JsonKeyAccepted]; !ok {
		return "", -1, util.ErrorFormat(errMissingJsonKey, paxos.JsonKeyAccepted)

	} else if accepted, ok := v.(string); !ok {
		return "", -1, util.ErrorFormat(errJsonValueType, v, stringType)

		/* Extract proposal number. */
	} else if p, ok := body[paxos.JsonKeyProposal]; !ok {
		return "", -1, util.ErrorFormat(errMissingJsonKey, paxos.JsonKeyProposal)

	} else if proposal, ok := p.(float64); !ok {
		return "", -1, util.ErrorFormat(errJsonValueType, p, stringType)

		/* Omit prepare statement for now.
		} else if p, ok := body[paxos.JsonKeyPrepare]; !ok {
			return "", -1, util.ErrorFormat(errMissingJsonKey, paxos.JsonKeyPrepare)

			} else if prepare, ok := p.(int); !ok {
				return "", -1, util.ErrorFormat(errJsonValueType, p, stringType)
		*/

	} else {
		return accepted, int(proposal), nil
	}
}

/* Get to proposer and return slice of addresses for accepters. */
func GetAccepters(host, port string) ([]string, error) {

	/* Format url. */
	addr := net.JoinHostPort(host, port)
	url := protocol + addr + paxos.GetAccepters

	/* Get to proposer. */
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	/* Body open on success; close when done. */
	defer resp.Body.Close()

	/* Assert OK. */
	if resp.StatusCode != http.StatusOK {
		return nil, unexpectedStatusCode(resp.StatusCode, http.StatusOK)
	}

	/* Body responses are json-formatted. */
	var body map[string]interface{}

	/* Decode body into map. */
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	/* Put addresses into slice from:
	 * { accepters : [ addr1, addr2, ... addrN ] } */
	if v, ok := body[paxos.JsonKeyAccepters]; !ok {
		return nil, util.ErrorFormat(errMissingJsonKey, paxos.JsonKeyAccepters)

	} else if I, ok := v.([]interface{}); !ok {
		return nil, util.ErrorFormat(errJsonValueType, v, stringType)

	} else {
		a := make([]string, len(I))
		for i := range I {
			if a[i], ok = I[i].(string); !ok {
				return nil, util.ErrorFormat(errJsonValueType, I[i], stringType)
			}
		}
		return a, nil
	}
}

/* Get to proposer and return slice of addresses for learners. */
func GetLearners(host, port string) ([]string, error) {

	/* Format url. */
	addr := net.JoinHostPort(host, port)
	url := protocol + addr + paxos.GetLearners

	/* Get to proposer. */
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	/* Body open on success; close when done. */
	defer resp.Body.Close()

	/* Assert OK. */
	if resp.StatusCode != http.StatusOK {
		return nil, unexpectedStatusCode(resp.StatusCode, http.StatusOK)
	}

	/* Body responses are json-formatted. */
	var body map[string]interface{}

	/* Decode body into map. */
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	/* Put addresses into slice from:
	 * { accepters : [ addr1, addr2, ... addrN ] } */
	if v, ok := body[paxos.JsonKeyLearners]; !ok {
		return nil, util.ErrorFormat(errMissingJsonKey, paxos.JsonKeyLearners)

	} else if I, ok := v.([]interface{}); !ok {
		return nil, util.ErrorFormat(errJsonValueType, v, stringType)

	} else {
		l := make([]string, len(I))
		for i := range I {
			if l[i], ok = I[i].(string); !ok {
				return nil, util.ErrorFormat(errJsonValueType, I[i], stringType)
			}
		}

		return l, nil
	}
}

/* Return formatted error from http codes. */
func unexpectedStatusCode(recv, expt int) error {
	recieved := http.StatusText(recv)
	expected := http.StatusText(expt)
	return util.ErrorFormat(errNotStatusOK, recv, recieved, expt, expected)
}
