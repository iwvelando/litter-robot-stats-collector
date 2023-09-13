// Package config defines the data structures related to configuration and
// includes functions for modifying the loading and parsing the config.
package config

import (
	"fmt"
	"github.com/spf13/viper"
	"time"
)

// Configuration holds all configuration for litter-robot-stats-collector
type Configuration struct {
	LitterRobot LitterRobot
	InfluxDB    InfluxDB
	Polling     Polling
}

// LitterRobot holds the connection parameters for the Litter API
type LitterRobot struct {
	Email    string
	Password string
}

// InfluxDB holds the connection parameters for InfluxDB
type InfluxDB struct {
	Address           string
	Username          string
	Password          string
	MeasurementPrefix string
	Database          string
	RetentionPolicy   string
	Token             string
	Organization      string
	Bucket            string
	SkipVerifySsl     bool
	FlushInterval     uint
}

// Polling holds parameters related to how we poll the Litter Robot
type Polling struct {
	Interval time.Duration
}

// LoadConfiguration takes a file path as input and loads the YAML-formatted
// configuration there.
func LoadConfiguration(configPath string) (*Configuration, error) {
	viper.SetConfigFile(configPath)
	viper.AutomaticEnv()

	viper.SetConfigType("yml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file, %s", err)
	}

	var configuration Configuration
	err := viper.Unmarshal(&configuration)
	if err != nil {
		return nil, fmt.Errorf("unable to decode into struct, %s", err)
	}

	return &configuration, nil
}
