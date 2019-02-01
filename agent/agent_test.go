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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
)

func TestTracingMethods(t *testing.T) {
	agent := Agent{
		name:        "test",
		spans:       make(map[string]opentracing.Span),
		collector:   nil,
		elastic:     nil,
		isDebugging: true,
	}

	var trace = TraceMessage{
		TraceId:   "5e27c67030932221",
		SpanId:    "38357d8f309b379d",
		Operation: "test",
	}

	span := agent.getSpan(trace)
	span.Finish()
	agent.freeSpan(trace)

	//Testing Trace API
	req, err := http.NewRequest("POST", "/trace", strings.NewReader(`{"parentSpanId":"5e27c67030932221", "traceId":"5e27c67030932221","spanId":"38357d8f309b379d","operation":"mysql-query","message":"select * from Patients"}`))
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(agent.Trace)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	//Testing Close API
	req, err = http.NewRequest("POST", "/close", strings.NewReader(`{"parentSpanId":"5e27c67030932221", "traceId":"5e27c67030932221","spanId":"38357d8f309b379d","operation":"mysql-query","message":"select * from Patients"}`))
	if err != nil {
		t.Fatal(err)
	}

	handler = http.HandlerFunc(agent.Close)

	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestTraceMessages(t *testing.T) {
	var trace = TraceMessage{}
	err := json.Unmarshal([]byte(`{"parentSpanId":"5e27c67030932221", "traceId":"5e27c67030932221","spanId":"38357d8f309b379d","operation":"mysql-query","message":"select * from Patients"}`), &trace)

	if err != nil {
		t.Error(err)
	}

	t.Logf("got %+v", trace)

	if trace.ParentSpanId != "5e27c67030932221" {
		t.Logf("traceMessage contained wrong value expected:%s got:%s", "5e27c67030932221", trace.TraceId)
		t.Fail()
	}

	if trace.TraceId != "5e27c67030932221" {
		t.Logf("traceMessage contained wrong value expected:%s got:%s", "5e27c67030932221", trace.TraceId)
		t.Fail()
	}

	if trace.SpanId != "38357d8f309b379d" {
		t.Logf("traceMessage contained wrong value expected:%s got:%s", "38357d8f309b379d", trace.TraceId)
		t.Fail()
	}

	if trace.Operation != "mysql-query" {
		t.Logf("traceMessage contained wrong value expected:%s got:%s", "mysql-query", trace.Operation)
		t.Fail()
	}

	if len(trace.Message) == 0 {
		t.Logf("expected any value got:%s", trace.Message)
		t.Fail()
	}
}

func TestMeterMessage(t *testing.T) {

	var meter = MeterMessage{}
	err := json.Unmarshal([]byte(`{"timestamp":"2018-02-19T12:32:32Z","value":9231.123,"unit":"byte per second"}`), &meter)

	if err != nil {
		t.Error(err)
	}

	t.Logf("got %+v", meter)

	if meter.Value != 9231.123 {
		t.Logf("MeterMessage contained wrong value expected:%f got:%f", 9231.123, meter.Value)
		t.Fail()
	}

	stamp, _ := time.Parse(time.RFC3339, "2018-02-19T12:32:32Z")

	if !meter.Timestamp.Equal(stamp) {
		t.Log("timestamps did not match")
		t.Fail()
	}

}

func TestLogMessages(t *testing.T) {

	var log = LogMessage{}
	err := json.Unmarshal([]byte(`{"timestamp":"2018-02-19T12:32:32Z","value":"foobar"}`), &log)

	if err != nil {
		t.Error(err)
	}

	t.Logf("got %+v", log)

	if log.Value != "foobar" {
		t.Logf("LogMessage contained wrong value expected:%s got:%s", "foobar", log.Value)
		t.Fail()
	}

	stamp, _ := time.Parse(time.RFC3339, "2018-02-19T12:32:32Z")

	if !log.Timestamp.Equal(stamp) {
		t.Log("timestamps did not match")
		t.Fail()
	}
}
