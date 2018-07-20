FROM golang:1.10.1 AS build
ENV SOURCEDIR=/go/src/github.com/DITAS-Project/VDC-Logging-Agent
RUN mkdir -p ${SOURCEDIR}
WORKDIR ${SOURCEDIR}
COPY . .
RUN rm -rf vendor/ && go get -u github.com/golang/dep/cmd/dep && dep ensure
#Patching opentracing
#RUN patch vendor/github.com/openzipkin/zipkin-go-opentracing/thrift/gen-go/scribe/scribe.go scribe.patch
RUN CGO_ENABLED=0 GOOS=linux go build -a --installsuffix cgo --ldflags="-s" -o loggingAgent

FROM alpine:3.4
COPY --from=build /go/src/github.com/DITAS-Project/VDC-Logging-Agent/loggingAgent /loggingAgent
ADD .config/logging.json .config/logging.json
EXPOSE 8484
CMD [ "./loggingAgent" ]