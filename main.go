package main

import (
	"flag"
	"fmt"
	"github.com/iwvelando/litter-robot-stats-collector/config"
	"github.com/iwvelando/litter-robot-stats-collector/influxdb"
	litterapi "github.com/tlkamp/litter-api"
	"go.uber.org/zap"
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

	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Println("{\"op\": \"main\", \"level\": \"fatal\", \"msg\": \"failed to initiate logger\"}")
		panic(err)
	}
	defer logger.Sync()

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
		logger.Fatal("failed to parse configuration",
			zap.String("op", "config.LoadConfiguration"),
			zap.Error(err),
		)
	}

	litterClient, err := litterapi.NewClient(&configuration.LitterRobot)
	litterClientExpiry := time.Now().Add(litterClient.Expiry - 1*time.Minute)
	if err != nil {
		logger.Fatal("failed to authenticate to Litter Robot",
			zap.String("op", "litter-api.NewClient"),
			zap.Error(err),
		)
	}

	influxClient, writeAPI, err := influxdb.Connect(configuration)
	if err != nil {
		logger.Fatal("failed to authenticate to InfluxDB",
			zap.String("op", "influxdb.Connect"),
			zap.Error(err),
		)
	}
	defer influxClient.Close()
	defer writeAPI.Flush()

	errorsCh := writeAPI.Errors()

	// Monitor InfluxDB write errors
	go func() {
		for err := range errorsCh {
			logger.Error("encountered error on writing to InfluxDB",
				zap.String("op", "influxdb.WriteAll"),
				zap.Error(err),
			)
		}
	}()

	// Look for SIGTERM or SIGINT
	cancelCh := make(chan os.Signal, 1)
	signal.Notify(cancelCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		for {

			if time.Now().After(litterClientExpiry) {
				litterClient.RefreshToken()
				litterClientExpiry = time.Now().Add(litterClient.Expiry - 1*time.Minute)
			}

			pollStartTime := time.Now()
			states, err := litterClient.States()
			queryTime := time.Now()
			if err != nil {
				logger.Fatal("failed to query Litter Robot states",
					zap.String("op", "litter-api.States"),
					zap.Error(err),
				)
			} else {
				influxdb.WriteAll(configuration, writeAPI, states, queryTime)
			}

			timeRemaining := configuration.Polling.Interval*time.Second - time.Since(pollStartTime)
			time.Sleep(time.Duration(timeRemaining))
			continue

		}
	}()

	sig := <-cancelCh
	logger.Info(fmt.Sprintf("caught signal %v, flushing data to InfluxDB", sig))
	writeAPI.Flush()

}
