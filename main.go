package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/iwvelando/litter-robot-stats-collector/config"
	"github.com/iwvelando/litter-robot-stats-collector/influxdb"
	log "github.com/sirupsen/logrus"
	lr "github.com/tlkamp/litter-api/v2/pkg/client"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var BuildVersion = "UNKNOWN"

// CliInputs holds the data passed in via CLI parameters
type CliInputs struct {
	BuildVersion string
	Config       string
	ShowVersion  bool
}

func main() {

	cliInputs := CliInputs{
		BuildVersion: BuildVersion,
	}
	flags := flag.NewFlagSet("litter-robot-stats-collector", 0)
	flags.StringVar(&cliInputs.Config, "config", "config.yaml", "Set the location for the YAML config file")
	flags.BoolVar(&cliInputs.ShowVersion, "version", false, "Print the version of modem-script")
	flags.Parse(os.Args[1:])

	if cliInputs.ShowVersion {
		fmt.Println(cliInputs.BuildVersion)
		os.Exit(0)
	}

	configuration, err := config.LoadConfiguration(cliInputs.Config)
	if err != nil {
		log.WithFields(log.Fields{
			"op":    "config.LoadConfiguration",
			"error": err,
		}).Fatal("failed to parse configuration")
	}

	api := lr.New(configuration.LitterRobot.Email, configuration.LitterRobot.Password)
	ctx := context.Background()

	err = api.Login(ctx)
	if err != nil {
		log.WithFields(log.Fields{
			"op":    "litter-api.Login",
			"error": err,
		}).Fatal("failed to authenticate to Litter API")
	}

	influxClient, writeAPI, err := influxdb.Connect(configuration)
	if err != nil {
		log.WithFields(log.Fields{
			"op":    "influxdb.Connect",
			"error": err,
		}).Fatal("failed to authenticate to InfluxDB")
	}
	defer influxClient.Close()
	defer writeAPI.Flush()

	errorsCh := writeAPI.Errors()

	// Monitor InfluxDB write errors
	go func() {
		for err := range errorsCh {
			log.WithFields(log.Fields{
				"op":    "influxdb.WriteAll",
				"error": err,
			}).Error("encountered error on writing to InfluxDB")
		}
	}()

	// Look for SIGTERM or SIGINT
	cancelCh := make(chan os.Signal, 1)
	signal.Notify(cancelCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		for {

			pollStartTime := time.Now()
			queryTime := time.Now()

			err = api.FetchRobots(ctx)
			if err != nil {
				log.WithFields(log.Fields{
					"op":    "litter-api.FetchRobots",
					"error": err,
				}).Fatal("failed to fetch robots from Litter API")
			} else {
				influxdb.WriteAll(configuration, writeAPI, api.Robots(), queryTime)
			}

			timeRemaining := configuration.Polling.Interval*time.Second - time.Since(pollStartTime)
			time.Sleep(time.Duration(timeRemaining))
			continue

		}
	}()

	sig := <-cancelCh
	log.WithFields(log.Fields{
		"op": "main",
	}).Info(fmt.Sprintf("caught signal %v, flushing data to InfluxDB", sig))
	writeAPI.Flush()

}
