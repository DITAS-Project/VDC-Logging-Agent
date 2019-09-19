/*
 * Copyright 2018 Information Systems Engineering, TU Berlin, Germany
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *                  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * This is being developed for the DITAS Project: https://www.ditas-project.eu/
 */

package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

func (agent *Agent) Trace(w http.ResponseWriter, req *http.Request) {
	log.Info("got trace request")

	if agent.tracing {
		var trace TraceMessage
		var body io.ReadCloser
		if agent.isDebugging {
			b, err := ioutil.ReadAll(req.Body)
			if err != nil {
				log.Errorf("could not write to elastic serach :%+v\n", err)
			}

			defer req.Body.Close()

			log.Debugln(string(b))

			body = ioutil.NopCloser(bytes.NewReader(b))

		} else {
			body = req.Body
		}
		err := json.NewDecoder(body).Decode(&trace)

		if err != nil {
			log.Errorf("faile dto read trace message %+v", err)
		}

		log.Infof("trace request for %s : %s", trace.ParentSpanId, trace.Operation)

		if agent.collector != nil {
			span := agent.getSpan(trace)
			if trace.Message != "" {
				span.LogEvent(trace.Message)
			}
		} else {
			log.Warn("tring to trace but no tracer set!")
		}
	}

	w.WriteHeader(200)
}

func (agent *Agent) Close(w http.ResponseWriter, req *http.Request) {

	log.Info("got trace finish request")

	if agent.tracing {
		var trace TraceMessage
		var body io.ReadCloser
		if agent.isDebugging {
			b, err := ioutil.ReadAll(req.Body)
			if err != nil {
				log.Errorf("could not write to elastic serach :%+v\n", err)
			}

			defer req.Body.Close()
			log.Debugln(string(b))

			body = ioutil.NopCloser(bytes.NewReader(b))

		} else {
			body = req.Body
		}
		err := json.NewDecoder(body).Decode(&trace)

		if err != nil {
			log.Errorf("faile dto read trace message %+v", err)
		}

		log.Infof("trace request for %s : %s", trace.ParentSpanId, trace.Operation)

		if agent.collector != nil {

			var span = agent.getSpan(trace)
			span.Finish()
			agent.freeSpan(trace)
		} else {
			log.Warn("tring to trace but no tracer set!")
		}
	}
	w.WriteHeader(200)
}

func (agent *Agent) Meter(w http.ResponseWriter, req *http.Request) {
	var meter MeterMessage
	_ = json.NewDecoder(req.Body).Decode(&meter)

	if agent.isDebugging {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorf("could not write to elastic serach :%+v\n", err)
		}

		defer req.Body.Close()
		log.Debugln(string(body))
		meter.Raw = string(body)
	}

	data := ElasticData{
		Timestamp: time.Now(),
		Meter:     &meter,
	}

	if (meter.Timestamp == time.Time{}) {
		data.Timestamp = time.Now()
	}

	agent.AddToES(data)

	w.WriteHeader(200)
}

func (agent *Agent) Log(w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)

	if err != nil {
		log.Errorf("could not write to elastic serach :%+v\n", err)
	}
	if agent.isDebugging {
		log.Debugln(string(body))
	}

	defer req.Body.Close()

	data := ElasticData{
		Timestamp: time.Now(),
		Log: &LogMessage{
			Value: string(body),
		},
	}

	agent.AddToES(data)

	w.WriteHeader(200)
}
