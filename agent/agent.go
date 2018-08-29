package agent

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/DITAS-Project/TUBUtil/util"
	"github.com/olivere/elastic"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	zipkin "github.com/openzipkin/zipkin-go-opentracing"
	"github.com/openzipkin/zipkin-go-opentracing/types"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var logger = logrus.New()
var log = logrus.NewEntry(logger)

func SetLogger(nLogger *logrus.Logger) {
	logger = nLogger
}

func SetLog(entty *logrus.Entry) {
	log = entty
}

type Configuration struct {
	Port int //the port of this service

	ZipkinEndpoint string //zipkin endpoint

	Endpoint string // the vdc endpoint
	VDCName  string // VDCName (used for the index name in elastic serach)

	ElasticSearchURL string //eleasticSerach endpoint

	waitTime time.Duration //the duration for which the server gracefully wait for existing connections to finish in secounds

}
type Agent struct {
	name        string
	spans       map[string]opentracing.Span
	collector   zipkin.Collector
	elastic     *elastic.Client
	isDebugging bool
	tracing     bool //if tracing should be loaded or not
}

func NewAgent() (*Agent, error) {

	err := viper.ReadInConfig()
	cnf := Configuration{}
	if err != nil {
		log.Error("failed to load config", err)
		return nil, err
	}
	viper.Unmarshal(&cnf)

	log.Info("config file used @ %s", viper.ConfigFileUsed())
	if viper.GetBool("verbose") {
		viper.Debug()
	}

	return CreateAgent(cnf)
}

func CreateAgent(cnf Configuration) (*Agent, error) {
	var collector zipkin.Collector
	if viper.GetBool("tracing") {
		// Create our HTTP collector.
		collector, err := zipkin.NewHTTPCollector(cnf.ZipkinEndpoint)

		if err != nil {
			log.Errorf("unable to create Zipkin HTTP collector: %+v\n", err)
			return nil, err
		}

		// Create our recorder.
		recorder := zipkin.NewRecorder(collector, viper.GetBool("verbose"), cnf.Endpoint, "vdc-agent")

		tracer, err := zipkin.NewTracer(recorder,
			zipkin.WithLogger(zipkin.LoggerFunc(func(kv ...interface{}) error {
				log.Info(kv)
				return nil
			})),
			zipkin.DebugMode(viper.GetBool("verbose")),
			zipkin.DebugAssertUseAfterFinish(viper.GetBool("verbose")),
			zipkin.DebugAssertUseAfterFinish(viper.GetBool("verbose")),
		)

		if err != nil {
			log.Errorf("unable to create Zipkin tracer: %+v\n", err)
			return nil, err
		}

		opentracing.InitGlobalTracer(tracer)
	}

	util.SetLogger(logger)
	util.SetLog(log)

	util.WaitForAvailible(cnf.ElasticSearchURL, nil)

	client, err := elastic.NewSimpleClient(
		elastic.SetURL(cnf.ElasticSearchURL),
		elastic.SetErrorLog(log),
		elastic.SetInfoLog(log),
	)
	if err != nil {
		log.Errorf("unable to create elastic client tracer: %+v\n", err)
		return nil, err
	}

	var ctx = Agent{
		name:        cnf.VDCName,
		spans:       make(map[string]opentracing.Span),
		collector:   collector,
		elastic:     client,
		isDebugging: viper.GetBool("verbose"),
		tracing:     viper.GetBool("tracing"),
	}

	// if err := ctx.init(); err != nil {
	// 	log.Errorf("unable to initilize this agent: %+v\n", err)
	// }

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
	Meter     *MeterMessage `json:"meter,omitempty"`
	Log       *LogMessage   `json:"log,omitempty"`
}

type MeterMessage struct {
	Timestamp   time.Time   `json:"timestamp,omitempty"`
	OperationID string      `json:"operationID,omitempty"`
	Value       interface{} `json:"value,omitempty"`
	Unit        string      `json:"unit,omitempty"`
	Name        string      `json:"name,omitempty"`
	Raw         string      `json:"appendix,omitempty"`
}

type LogMessage struct {
	Timestamp time.Time `json:"timestamp,omitempty"`
	Value     string    `json:"value,omitempty"`
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

func (agent *Agent) InitES() error {
	ctx := context.Background()
	mappings := `
			"properties": {
				"@timestamp": {
					"type": "date"
				},
				"meter": {
					"properties": {
						"meter.timestamp": {
							"type": "date"
						},
						"meter.unit": {
							"type": "text"
						},
						"meter.value": {
							"type": "text"
						}
						"meter.name": {
							"type": "text"
						},
						"meter.appendix": {
							"type": "text"
						},
						"meter.operationID": {
							"type": "keyword"
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
	return util.GetElasticIndex(agent.name)
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

func (agent *Agent) AddToES(data ElasticData) error {

	ctx := context.Background()
	if _, err := agent.elastic.Index().
		Index(agent.getElasticIndex()).
		Type("data").
		BodyJson(data).
		Do(ctx); err != nil {

		log.Errorf("could not write to elastic serach :%+v\n", err)
		return err
	}

	return nil
}
