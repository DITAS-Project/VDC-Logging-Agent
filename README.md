# DITAS - VDC Request Monitor

The VDC Logging agent is a small software service to enable a VDC to transmit metrics and instrumentation information to the DITAS platform.
The agent offers a rest interface to each VDC, which enables access to elastic search and Zipkin without requiring the VDC to included specific dependencies for these services.
The agent is intended to run in the same container as the VDC and is compiled with static libraries, allowing it to be deployed in any Unix-like environment.

## Getting Started

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes. See deployment for notes on how to deploy the project on a live system.

### Prerequisites

To install the go lang tools go to: [Go Getting Started](https://golang.org/doc/install)


To install dep, you can use this command or go to [Dep - Github](https://github.com/golang/dep):
```
curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
```

### Installing

For installation you have two options, building and running it on your local machine or using the docker approach.

For local testing and building for that you can follow the following steps:

install dependencies (only needs to be done once):

```
dep ensure
```

compile
```
CGO_ENABLED=0 GOOS=linux go build -a --installsuffix cgo --ldflags="-w -s -X main.Build=$(git rev-parse --short HEAD)" -o log-agnt
```

to run locally:
```
./log-agnt
```

For the docker approach, you can use the provided dockerfile to build a running artifact as a Docker container.

build the docker container:
```
docker build -t ditas/logging-agent -f Dockerfile.artifact . 
```

Attach the docker container to a VDC or other microservice like component:
```
docker run -v ./logging.json:/opt/blueprint/logging.json --pid=container:<APPID> -p 8484:8484 ditas/logging-agent 
```
Here `<APPID>` must be the container ID of the application you want to observe. The port at 8484 is used for the logging rest interface and only needs to be exposed if you plan on logging data of external services outside the attached container. Also, refer to the **Configuration** section for information about the `logging.json`-config file.

## Running the tests

For testing you can use:
```
 go test ./...
```

For that make sure you have an elastic search running locally at the default port. 


## Configuration
To configure the agent, you can specify the following values in a JSON file:
 * ElasticSearchURL => The URL that all aggregated data is sent to
 * VDCName => the Name used to store the information under
 * Endpoint => the address used as the service address in zipkin
 * ZipkinEndpoint => the address of the zipkin collector
 * tracing => boolean that indicates if tracing should be enabled 
 * Port => port of the agent
 * verbose => boolean to indicate if the agent should use verbose logging (recommended for debugging)

An example file could look like this:
```
{
    "Port":8484,
    "ElasticSearchURL":"http://127.0.0.1:9200",
    "VDCName":"tubvdc",
    "Endpoint":"http://127.0.0.1:8080",
    "verbose":false
}
```

Alternatively, users can use flags with the same name to configure the agent.

#### V01
The command line options are:
 - ``-port`` port that the agent should listen on
 - ``-zipkin`` Zipkin endpoint 
 - ``-vdc``  VDC name that this agent is paired with (used as the elastic search index)
 - ``-elastic`` elastic search address
 - ``-wait`` the duration for which the server gracefully wait for existing connections in seconds

### API
This agent offers a logging API that can be used by attached applications to forward important information to the DITAS monitoring system.

An excerpt of the version 1.0.0 API can be found (here)[https://github.com/DITAS-Project/VDC-Logging-Agent/blob/master/api/swagger.v1.yml]. 

## Built With

* [dep](https://github.com/golang/dep)
* [viper](https://github.com/spf13/viper)
* Zipkin
* OpenTracing
* [ElasticSearch](https://www.elastic.co/)

## Versioning

We use [SemVer](http://semver.org/) for versioning. For the versions available, see the [tags on this repository](https://github.com/your/project/tags). 

## License

This project is licensed under the Apache 2.0 - see the [LICENSE.md](LICENSE.md) file for details.

## Acknowledgments

This is being developed for the [DITAS Project](https://www.ditas-project.eu/)
