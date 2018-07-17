package agent

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"
)

func (agent *Agent) Trace(w http.ResponseWriter, req *http.Request) {
	log.Info("got trace request")

	if agent.tracing {
		var trace TraceMessage
		_ = json.NewDecoder(req.Body).Decode(&trace)

		log.Infof("trace request for %s : %s", trace.SpanId, trace.Operation)

		span := agent.getSpan(trace)

		if trace.Message != "" {
			span.LogEvent(trace.Message)
		}
	}
}

func (agent *Agent) Close(w http.ResponseWriter, req *http.Request) {
	log.Info("got trace finish request")

	if agent.tracing {
		var trace TraceMessage
		_ = json.NewDecoder(req.Body).Decode(&trace)

		log.Infof("trace request for %s : %s", trace.SpanId, trace.Operation)

		var span = agent.getSpan(trace)
		span.Finish()
		agent.freeSpan(trace)
	}
}

func (agent *Agent) Meter(w http.ResponseWriter, req *http.Request) {
	var meter MeterMessage
	_ = json.NewDecoder(req.Body).Decode(&meter)

	if agent.isDebugging {
		//TODO: put request body into meter.raw
	}
	data := ElasticData{
		Timestamp: time.Now(),
		Meter:     &meter,
	}
	if (meter.Timestamp == time.Time{}) {
		data.Timestamp = time.Now()
	}

}

func (agent *Agent) Log(w http.ResponseWriter, req *http.Request) {

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Errorf("could not write to elastic serach :%+v\n", err)
	}

	defer req.Body.Close()

	data := ElasticData{
		Timestamp: time.Now(),
		Log: &LogMessage{
			Value: string(body),
		},
	}
	agent.AddToES(data)
}
