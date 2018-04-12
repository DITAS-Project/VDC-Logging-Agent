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
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/DITAS-Project/VDC-Logging-Agent/agent"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func main() {
	//read cmd options
	portPtr := flag.Int("port", 8484, "port that the agent should listen on")
	zipkinAddressPtr := flag.String("zipkin", "http://localhost:9411/api/v1/spans", "zipkin address")
	vdcAddressPtr := flag.String("vdc", "http://0.0.0.0:0", "vdc address to be send to zipkin")
	vdcName := flag.String("name", "vdc", "vdc name that this agent is paired with (used as the elastic search index)")
	elasticAddress := flag.String("elastic", "http://127.0.0.1:9200", "elastic search address")
	var wait time.Duration
	flag.DurationVar(&wait, "wait", 15, "the duration for which the server gracefully wait for existing connections to finish in secounds")

	flag.Parse()

	log.SetLevel(logrus.DebugLevel)

	agent, err := agent.NewAgent(*vdcName, *zipkinAddressPtr, *elasticAddress, true, *vdcAddressPtr)

	if err != nil {
		log.Errorf("Failed to init agent %s", err)
		os.Exit(-1)
	}

	startServer(agent, *portPtr, wait)

}

func startServer(agent *agent.Agent, port int, waitTime time.Duration) {
	//setup routing
	apiRouter := mux.NewRouter()
	apiRouter.NotFoundHandler = http.HandlerFunc(notFound)

	v1 := apiRouter.PathPrefix("/v1").Subrouter()
	v1.PathPrefix("/close").Methods("POST").Handler(http.HandlerFunc(agent.Close))
	v1.PathPrefix("/trace").Methods("PUT").Handler(http.HandlerFunc(agent.Trace))

	v1.PathPrefix("/meter").Methods("POST").Handler(http.HandlerFunc(agent.Meter))
	v1.PathPrefix("/log").Methods("POST").Handler(http.HandlerFunc(agent.Log))

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
			log.Error(err)
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
