package agent

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/olivere/elastic"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	zipkin "github.com/openzipkin/zipkin-go-opentracing"
	"github.com/openzipkin/zipkin-go-opentracing/types"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

type Agent struct {
	name        string
	spans       map[string]opentracing.Span
	collector   zipkin.Collector
	elastic     *elastic.Client
	isDebugging bool
}

func NewAgent(name string, zipkinAddressPtr string, elasticAddress string, debug bool, vdcAddress string) (*Agent, error) {
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

	var ctx = Agent{
		name:        name,
		spans:       make(map[string]opentracing.Span),
		collector:   collector,
		elastic:     client,
		isDebugging: debug,
	}

	if err := ctx.init(); err != nil {
		log.Errorf("unable to initilize this agent: %+v\n", err)
	}

	return &ctx, nil
}

func (agent *Agent) Shutdown() {
	agent.collector.Close()
	agent.elastic.Stop()
}

type TraceMessage struct {
	TraceId      string `json:"traceId"`
	ParentSpanId string `json:"parentSpanId"`
	SpanId       string `json:"spanId"`
	Operation    string `json:"operation"`
	Message      string `json:"message"`
}

type ElasticData struct {
	Timestamp time.Time     `json:"@timestamp"`
	Meter     *MeterMessage `json:"meter"`
	Log       *LogMessage   `json:"log"`
}

type MeterMessage struct {
	Timestamp time.Time `json:"@timestamp"`
	Value     float64   `json:"meter.value"`
	Unit      string    `json:"meter.unit"`
	Raw       string    `json:"meter.appendix"`
}

type LogMessage struct {
	Timestamp time.Time `json:"@timestamp"`
	Value     string    `json:"log.value"`
}

//tracing functions
func (t TraceMessage) build() *zipkin.SpanContext {
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

func (agent *Agent) init() error {
	ctx := context.Background()
	mappings := `
			"properties": {
				"@timestamp": {
					"type": "date"
				},
				"meter": {
					"properties": {
						"meter.value": {
							"type": "double"
						},
						"meter.unit": {
							"type": "text"
						},
						"meter.appendix": {
							"type": "object"
						}
					}
				},
				"log": {
					"properties": {
						"log.value": {
							"type": "text"
						}
					}
				}
			}
		`

	if ok, err := agent.elastic.IndexExists(agent.getElasticIndex()).Do(ctx); !ok || err != nil {
		log.Infof("creating inxex %s", agent.getElasticIndex())

		index := fmt.Sprintf(`{
			"settings": {
				"number_of_shards": 1,
				"number_of_replicas": 0
			},
			"mappings": {
				"data": {
					%s
				}
			}
		}`, mappings)

		_, err := agent.elastic.CreateIndex(agent.getElasticIndex()).BodyString(index).Do(ctx)
		if err != nil {
			return err
		}
		return nil

	} else {
		_, err := agent.elastic.PutMapping().
			Index(agent.getElasticIndex()).
			Type("data").
			BodyString(fmt.Sprintf("{%s},", mappings)).
			Do(ctx)
		if err != nil {
			return err
		}
		return nil
	}

}

func (agent *Agent) getElasticIndex() string {
	t := time.Now()
	return fmt.Sprintf("%s-%d-%02d-%02d", agent.name, t.Year(), t.Month(), t.Day())
}

func (agent *Agent) getSpan(trace TraceMessage) opentracing.Span {

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

func (agent *Agent) freeSpan(trace TraceMessage) {
	delete(agent.spans, trace.TraceId+trace.SpanId)
}
