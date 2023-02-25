package main

import (
	"flag"
	"os"

	"github.com/marius-j-i/paxos/client"
	log "github.com/sirupsen/logrus"
)

/* Structure representing arguments from command-line. */
type CmdArg struct {
	host, port string
	value      string
}

func main() {

	cmdarg, err := parseCmdline()
	if err != nil {
		exit(err)
	}

	err = client.Propose(cmdarg.host, cmdarg.port, cmdarg.value)
	if err != nil {
		exit(err)
	}
}

/* Return struct representing arguments from command-line. */
func parseCmdline() (*CmdArg, error) {
	cmdarg := &CmdArg{}

	/* Closure creating value struct into cmdarg. */
	value := func(path string) error {
		v, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		cmdarg.value = string(v)
		return nil
	}

	args := flag.NewFlagSet("args", flag.ExitOnError)

	args.StringVar(&cmdarg.host, "host", "",
		"Required: Resolvable hostname for machine to reach a proposer")
	args.StringVar(&cmdarg.port, "port", "",
		"Required: Port for proposer process on -host")
	args.Func("value",
		"Required: Path to file where value for proposer is", value)

	args.Parse(os.Args[1:])

	/* Required arguments. */
	if cmdarg.host == "" || cmdarg.port == "" || cmdarg.value == "" {
		args.Usage()
		exit(nil)
	}

	return cmdarg, nil
}

/* Exit program with error code 1 if err is non-nil, otherwise code is 0. */
func exit(err error) {
	code := 0
	if err != nil {
		log.Error(err)
		code = 1
	}
	os.Exit(code)
}

/* Struct for proposing value. */
type Value struct {
	/* Path to file to read value from. */
	path string
	/* Nil when closed. */
	file *os.File
}

/* Return new value. */
func newValue(path string) (*Value, error) {

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &Value{
		path: path,
		file: f,
	}, nil
}

/* Read and copy file into argument.
 * Implements Reader interface. */
func (v *Value) Read(p []byte) (n int, err error) {
	return v.file.Read(p)
}

/* Close file if still open.
 * Implements Closer interface. */
func (v *Value) Close() error {
	return v.file.Close()
}
