
# An implementation of Paxos 

## Inspired by Leslie Lamport's Paxos Made Simple

## Paxos Client

The client features a simplistic command-line interface to post a value from a file to a paxos client. 

### Tests

Some pre-configured tests are available and can be run with `go test ./client`

### Paxos Proposer API's

The client assumes the paxos proposer server is running and is reachable at the following end-points

* POST `/propose/<str-value>`: Initiates a proposal to be accepted. Status code for a successfull request is 201 CREATED on proposal achieving quorum.
* GET `/accepted`: Returns the currently accepted value with its corresponding proposal number. Status code for successful request is 200 OK.
* GET `/accepters`: Returns the currently available accepters in the network. Status code for successfull request is 200 OK.
* GET `/learners`: Returns the currently available learners in the network, Status code for successfull request is 200 OK.

The client assumes the same (and consistent information) is reachable at different proposers in the network.

### Building the Client

From the same directory as this readme-file, do; 
`go build` or `go build main.go`.

### Running the Client

From the same directory as this readme-file, either; 
`go run -host <paxos-machine> -port <int> -value <file-path>` or 
`go build && ./paxos-client <args...>`

### Client Usage

For usage information, supply either `-h` or `-help` flags when running the client.

NOTE: Flag prefixes with double or single dashes are equivalent regardless of short-hand or verbose flag, e.g., `-host` = `--host` and `--h` = `--help`

