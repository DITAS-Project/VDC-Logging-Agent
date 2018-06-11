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
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func main() {

	viper.SetConfigName("logging")
	viper.AddConfigPath("/.config/")
	viper.AddConfigPath(".config/")
	viper.AddConfigPath(".")

	viper.SetDefault("Port", 8484)
	viper.SetDefault("ZipkinEndpoint", "http://localhost:9411/api/v1/spans")
	viper.SetDefault("VDCName", "dummyVDC")
	viper.SetDefault("ElasticSearchURL", "http://127.0.0.1:9200")
	viper.SetDefault("waitTime", time.Duration(1.5e+10))
	viper.SetDefault("verbose", false)

	viper.RegisterAlias("zipkin", "ZipkinEndpoint")
	viper.RegisterAlias("vdc", "Endpoint")
	viper.RegisterAlias("name", "VDCName")
	viper.RegisterAlias("elastic", "ElasticSearchURL")

	//read cmd options
	flag.Bool("verbose", false, "for debugging and logging")
	flag.Int("Port", viper.GetInt("Port"), "port that the agent should listen on")
	flag.String("zipkin", "http://localhost:9411/api/v1/spans", "zipkin address")
	flag.String("vdc", "http://0.0.0.0:0", "vdc address to be send to zipkin")
	flag.String("name", "vdc", "vdc name that this agent is paired with (used as the elastic search index)")
	flag.String("elastic", "http://127.0.0.1:9200", "elastic search address")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	log.SetLevel(logrus.DebugLevel)

	agent, err := agent.NewAgent()

	if err != nil {
		log.Errorf("Failed to init agent %s", err)
		os.Exit(-1)
	}

	startServer(agent, viper.GetInt("Port"), viper.GetDuration("waitTime"))

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
