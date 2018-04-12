package agent

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"
)

func (agent *Agent) Trace(w http.ResponseWriter, req *http.Request) {
	log.Info("got trace request")

	var trace TraceMessage
	_ = json.NewDecoder(req.Body).Decode(&trace)

	log.Infof("trace request for %s : %s", trace.SpanId, trace.Operation)

	span := agent.getSpan(trace)

	if trace.Message != "" {
		span.LogEvent(trace.Message)
	}
}

func (agent *Agent) Close(w http.ResponseWriter, req *http.Request) {
	log.Info("got trace finish request")

	var trace TraceMessage
	_ = json.NewDecoder(req.Body).Decode(&trace)

	log.Infof("trace request for %s : %s", trace.SpanId, trace.Operation)

	var span = agent.getSpan(trace)
	span.Finish()
	agent.freeSpan(trace)
}

func (agent *Agent) Meter(w http.ResponseWriter, req *http.Request) {
	var meter MeterMessage
	_ = json.NewDecoder(req.Body).Decode(&meter)

	if agent.isDebugging {
		//TODO: put request body into meter.raw
	}

	if (meter.Timestamp == time.Time{}) {
		meter.Timestamp = time.Now()
	}

	ctx := context.Background()
	if _, err := agent.elastic.Index().
		Index(agent.getElasticIndex()).
		Type("meter").
		BodyJson(meter).
		Do(ctx); err != nil {
		log.Errorf("could not write to elastic serach :%+v\n", err)
	}
}

func (agent *Agent) Log(w http.ResponseWriter, req *http.Request) {

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Errorf("could not write to elastic serach :%+v\n", err)
	}

	defer req.Body.Close()

	lm := LogMessage{
		Timestamp: time.Now(),
		Value:     string(body),
	}

	ctx := context.Background()
	if _, err := agent.elastic.Index().
		Index(agent.getElasticIndex()).
		Type("log").
		BodyJson(lm).
		Do(ctx); err != nil {
		log.Errorf("could not write to elastic serach :%+v\n", err)
	}
}
