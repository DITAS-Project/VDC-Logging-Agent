
# VDC-Logging-Agent

The VDC Logging agent is a small software service to enable a VDC to transmit metrics and instrumentation information to the DITAS platform.

The agent offers a rest interface to each VDC, which enables access to elastic search and Zipkin without requiring the VDC to included specific dependencies for these services.

The agent is intended to run in the same container as the VDC and is compiled with static libraries, allowing it to be deployed in any Unix-like environment.

## Usage
The agent offers a set of command line arguments that are needed to pair an agent with a given VDC. 

#### V01
The command line options are:
 - ``-port`` port that the agent should listen on
 - ``-zipkin`` Zipkin endpoint 
 - ``-vdc``  VDC name that this agent is paired with (used as the elastic search index)
 - ``-elastic`` elastic search address
 - ``-wait`` the duration for which the server gracefully wait for existing connections in seconds

## Development
We use ``dep`` for dependencies management and otherwise, standart go.

To compile a executebale that can be inculde in a container, we use the following command:
```shell
docker run --rm -it -v "$GOPATH:/gopath" -v "$(pwd):/app" -v "$(pwd)/vendor:/vendor/src" -e "GOPATH=/gopath:/vendor" -w /app golang:1.8.3 sh -c 'CGO_ENABLED=0 go build -a --installsuffix cgo --ldflags="-s" -o vdc-agent'
```
