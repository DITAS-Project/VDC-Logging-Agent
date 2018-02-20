/*
	Copyright 2018 TU Berlin - ISE

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

		http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/olivere/elastic"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	zipkin "github.com/openzipkin/zipkin-go-opentracing"
	"github.com/openzipkin/zipkin-go-opentracing/types"
	"io/ioutil"
)

var log = logrus.New()

func main() {
	//read cmd options
	portPtr := flag.Int("port", 8484, "port that the agent should listen on")
	zipkinAddressPtr := flag.String("zipkin", "http://localhost:9411/api/v1/spans", "zipkin address")
	vdcAddressPtr := flag.String("vdc", "http://0.0.0.0:0", "vdc address to be send to zipkin")
	vdcName := flag.String("name", "vdc", "vdc name that this agent is paired with (used as the elastic search index)")
	elasticAddress := flag.String("elastic","http://127.0.0.1:9200","elastic search address")
	var wait time.Duration
	flag.DurationVar(&wait, "wait", 15, "the duration for which the server gracefully wait for existing connections to finish in secounds")

	flag.Parse()

	log.SetLevel(logrus.DebugLevel)

	agent, err := newAgent(*vdcName,*zipkinAddressPtr,*elasticAddress, true, *vdcAddressPtr)

	if err != nil {
		log.Errorf("Failed to init agent %s", err)
		os.Exit(-1)
	}

	startServer(agent, *portPtr, wait)

}

func startServer(agent *agent, port int, waitTime time.Duration) {
	//setup routing
	apiRouter := mux.NewRouter()
	apiRouter.NotFoundHandler = http.HandlerFunc(notFound)

	v1 := apiRouter.PathPrefix("/v1").Subrouter()
	v1.PathPrefix("/close").Methods("POST").Handler(http.HandlerFunc(agent.close))
	v1.PathPrefix("/trace").Methods("PUT").Handler(http.HandlerFunc(agent.trace))

	v1.PathPrefix("/meter").Methods("POST").Handler(http.HandlerFunc(agent.meter))
	v1.PathPrefix("/log").Methods("POST").Handler(http.HandlerFunc(agent.log))

	//start server
	api := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      apiRouter,
	}

	go func() {
		log.Infof("Listening on :%d", port)
		if err := api.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	//gracefull shutdown @see mux github.com
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c

	ctx, cancel := context.WithTimeout(context.Background(), waitTime)
	defer cancel()
	agent.Shutdown()
	api.Shutdown(ctx)
	log.Info("shutting down")
	os.Exit(0)
}

func notFound(w http.ResponseWriter, req *http.Request) {
	log.Infof("request not found", req.URL)
}

//agent section

type agent struct {
	name string
	spans     map[string]opentracing.Span
	collector zipkin.Collector
	elastic *elastic.Client
	isDebugging bool
}

func newAgent(name string, zipkinAddressPtr string,elasticAddress string, debug bool, vdcAddress string) (*agent, error) {
	// Create our HTTP collector.
	collector, err := zipkin.NewHTTPCollector(zipkinAddressPtr)

	if err != nil {
		log.Errorf("unable to create Zipkin HTTP collector: %+v\n", err)
		return nil, err
	}

	// Create our recorder.
	recorder := zipkin.NewRecorder(collector, debug, vdcAddress, "vdc-agent")

	tracer, err := zipkin.NewTracer(recorder,
		zipkin.WithLogger(zipkin.LoggerFunc(func(kv ...interface{}) error {
			log.Info(kv)
			return nil
		})),
		zipkin.DebugMode(debug),
		zipkin.DebugAssertUseAfterFinish(debug),
		zipkin.DebugAssertUseAfterFinish(debug),
	)

	if err != nil {
		log.Errorf("unable to create Zipkin tracer: %+v\n", err)
		return nil, err
	}

	opentracing.InitGlobalTracer(tracer)


	client, err := elastic.NewSimpleClient(
		elastic.SetURL(elasticAddress),
		elastic.SetErrorLog(log),
		elastic.SetInfoLog(log),
	)
	if err != nil {
		log.Errorf("unable to create elastic client tracer: %+v\n", err)
		return nil, err
	}

	var ctx = agent{
		name:	name,
		spans:     make(map[string]opentracing.Span),
		collector: collector,
		elastic: client,
		isDebugging: debug,
	}
	
	
	if _,err := ctx.init();err != nil {
		log.Errorf("unable to initilize this agent: %+v\n", err)
		return nil, err
	}

	return &ctx, nil
}


func (agent *agent) Shutdown() {
	agent.collector.Close()
	agent.elastic.Stop()
}

type traceMessage struct {
	TraceId      string `json:"traceId"`
	ParentSpanId string `json:"parentSpanId"`
	SpanId       string `json:"spanId"`
	Operation    string `json:"operation"`
	Message      string `json:"message"`
}

type meterMessage struct {
	timestamp time.Time `json:"@timestamp"`
	value float64 `json:"value"`
	unit string `json:"unit"`
	raw string `json:"appendix"`
}

type logMessage struct {
	timestamp time.Time `json:"@timestamp"`
	value string `json:"value"`
}


//tracing functions
func (t traceMessage) build() *zipkin.SpanContext {
	var pid *uint64
	ppid, err := strconv.ParseUint(t.ParentSpanId, 16, 64)
	if err != nil {
		pid = nil
	} else {
		pid = &ppid
	}
	sid, err := strconv.ParseUint(t.SpanId, 16, 64)
	if err != nil {
		log.Errorf("did not parse sid %s - %s", t.SpanId, err)
		return nil
	}

	tid, err := types.TraceIDFromHex(t.TraceId)
	if err != nil {
		log.Errorf("did not parse tid %s - %s", t.TraceId, err)
		return nil
	}

	context := zipkin.SpanContext{
		TraceID:      tid,
		ParentSpanID: pid,
		SpanID:       sid,
		Sampled:      true,
	}

	return &context
}

func (agent *agent) init() (*elastic.IndicesCreateResult,error) {
	mapping := `{
		"settings":{
			"number_of_shards":1,
			"number_of_replicas":0
		},
		"mappings":{
			"metic":{
				"properties":{
					"@timestamp":{
						"type":"date"
					},
					"metic.value":{
						"type":"double"
					},
					"metic.unit":{
						"type":"text"
					},
					"metic.appendix":{
						"type":"object"
					}
					
				}
			},
			"log":{
				"properties":{
					"@timestamp":{
						"type":"date"
					},
					"log.value":{
						"type":"object"
					}
				}
			}
		}
	}`
	
	ctx := context.Background()
	createIndex, err := agent.elastic.CreateIndex(agent.getElasticIndex()).BodyString(mapping).Do(ctx)
	if err != nil {
		return nil,err
	}

	return createIndex,nil
}

func (agent *agent) getElasticIndex() string {
	t := time.Now()
	return fmt.Sprintf("%s-%d-%02d-%02d",agent.name,t.Year(),t.Month(),t.Day())
}

func (agent *agent) getSpan(trace traceMessage) opentracing.Span {

	if span, ok := agent.spans[trace.TraceId+trace.SpanId]; ok {
		log.Infof("updateing trace %s", trace.SpanId)
		return span
	}

	log.Infof("building trace %s", trace.SpanId)
	var context = trace.build()

	if context != nil {
		span := opentracing.StartSpan(trace.Operation, ext.RPCServerOption(*context))
		agent.spans[trace.TraceId+trace.SpanId] = span
		log.Infof("trace %s build", trace.SpanId)
		return span
	}

	return opentracing.StartSpan(trace.Operation)

}

func (agent *agent) freeSpan(trace traceMessage) {
	delete(agent.spans,trace.TraceId+trace.SpanId)
}

func (agent *agent) trace(w http.ResponseWriter, req *http.Request) {
	log.Info("got trace request")

	var trace traceMessage
	_ = json.NewDecoder(req.Body).Decode(&trace)

	log.Infof("trace request for %s : %s", trace.SpanId, trace.Operation)

	span := agent.getSpan(trace)

	if trace.Message != "" {
		span.LogEvent(trace.Message)
	}
}

func (agent *agent) close(w http.ResponseWriter, req *http.Request) {
	log.Info("got trace finish request")

	var trace traceMessage
	_ = json.NewDecoder(req.Body).Decode(&trace)

	log.Infof("trace request for %s : %s", trace.SpanId, trace.Operation)

	var span = agent.getSpan(trace)
	span.Finish()
	agent.freeSpan(trace);
}

func (agent *agent) meter(w http.ResponseWriter, req *http.Request) {
	var meter meterMessage
	_ = json.NewDecoder(req.Body).Decode(&meter)

	if(agent.isDebugging){
		//TODO put request body into meter.raw
	}

	if (meter.timestamp == time.Time{}){
		meter.timestamp = time.Now()
	}
	

	ctx := context.Background()
	if _, err := agent.elastic.Index().
		Index(agent.getElasticIndex()).
		Type("meter").
		BodyJson(meter).
		Do(ctx); err != nil {
		log.Errorf("could not write to elastic serach :%+v\n",err)
	}
}

func (agent *agent) log(w http.ResponseWriter, req *http.Request) {
	
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Errorf("could not write to elastic serach :%+v\n",err)
	}
	
	defer req.Body.Close()

	lm := logMessage{
		timestamp: time.Now(),
		value: string(body),
	}

	ctx := context.Background()
	if _, err := agent.elastic.Index().
		Index(agent.getElasticIndex()).
		Type("log").
		BodyJson(lm).
		Do(ctx); err != nil {
		log.Errorf("could not write to elastic serach :%+v\n",err)
	}
}