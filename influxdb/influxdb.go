package influxdb

import (
	"crypto/tls"
	"fmt"
	influx "github.com/influxdata/influxdb-client-go/v2"
	influxAPI "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/iwvelando/litter-robot-stats-collector/config"
	lr "github.com/tlkamp/litter-api/v2/pkg/client"
	"math"
	"time"
)

// LitterRobotState describes the state with types we want to use for InfluxDB
type LitterRobotState struct {
	LitterRobotSerial         string
	Name                      string
	LitterRobotID             string
	PowerStatus               string
	CycleCount                int
	CyclesUntilFull           int
	DFICycleCount             int
	CyclesAfterDrawerFull     int
	CycleCapacity             int
	CleanCycleWaitTimeMinutes int
	UnitStatus                int
	DFITriggered              bool
	NightLightActive          bool
	PanelLockActive           bool
	DidNotifyOffline          bool
	SleepModeActive           bool
}

type InfluxWriteConfigError struct{}

func (r *InfluxWriteConfigError) Error() string {
	return "must configure at least one of bucket or database/retention policy"
}

func Connect(config *config.Configuration) (influx.Client, influxAPI.WriteAPI, error) {
	var auth string
	if config.InfluxDB.Token != "" {
		auth = config.InfluxDB.Token
	} else if config.InfluxDB.Username != "" && config.InfluxDB.Password != "" {
		auth = fmt.Sprintf("%s:%s", config.InfluxDB.Username, config.InfluxDB.Password)
	} else {
		auth = ""
	}

	var writeDest string
	if config.InfluxDB.Bucket != "" {
		writeDest = config.InfluxDB.Bucket
	} else if config.InfluxDB.Database != "" && config.InfluxDB.RetentionPolicy != "" {
		writeDest = fmt.Sprintf("%s/%s", config.InfluxDB.Database, config.InfluxDB.RetentionPolicy)
	} else {
		return nil, nil, &InfluxWriteConfigError{}
	}

	if config.InfluxDB.FlushInterval == 0 {
		config.InfluxDB.FlushInterval = 30
	}

	options := influx.DefaultOptions().
		SetFlushInterval(1000 * config.InfluxDB.FlushInterval).
		SetTLSConfig(&tls.Config{
			InsecureSkipVerify: config.InfluxDB.SkipVerifySsl,
		})
	client := influx.NewClientWithOptions(config.InfluxDB.Address, auth, options)

	writeAPI := client.WriteAPI(config.InfluxDB.Organization, writeDest)

	return client, writeAPI, nil
}

func WriteAll(config *config.Configuration, writeAPI influxAPI.WriteAPI, robots []lr.Robot, ts time.Time) error {

	for _, robot := range robots {
		litterRobotState := CleanState(robot)
		p := influx.NewPoint(
			config.InfluxDB.MeasurementPrefix+"litter_robot",
			map[string]string{
				"robot_id":     litterRobotState.LitterRobotID,
				"robot_serial": litterRobotState.LitterRobotSerial,
				"robot_name":   litterRobotState.Name,
			},
			map[string]interface{}{
				"clean_cycle_wait_time_minutes": litterRobotState.CleanCycleWaitTimeMinutes,
				"cycles_after_drawer_full":      litterRobotState.CyclesAfterDrawerFull,
				"cycles_capacity":               litterRobotState.CycleCapacity,
				"cycles_count":                  litterRobotState.CycleCount,
				"cycles_until_full":             litterRobotState.CyclesUntilFull,
				"did_notify_offline":            litterRobotState.DidNotifyOffline,
				"dfi_cycle_count":               litterRobotState.DFICycleCount,
				"dfi_triggered":                 litterRobotState.DFITriggered,
				"night_light_active":            litterRobotState.NightLightActive,
				"panel_lock_active":             litterRobotState.PanelLockActive,
				"power_status":                  litterRobotState.PowerStatus,
				"sleep_mode_active":             litterRobotState.SleepModeActive,
				"unit_status":                   litterRobotState.UnitStatus,
			},
			ts)

		writeAPI.WritePoint(p)
	}

	return nil
}

func CleanState(r lr.Robot) (robot LitterRobotState) {

	robot.CleanCycleWaitTimeMinutes = fToI(r.CleanCycleWaitTimeMinutes)
	robot.CyclesAfterDrawerFull = fToI(r.CyclesAfterDrawerFull)
	robot.CycleCapacity = fToI(r.CycleCapacity)
	robot.CycleCount = fToI(r.CycleCount)
	robot.CyclesUntilFull = fToI(r.CyclesUntilFull)
	robot.DidNotifyOffline = r.DidNotifyOffline
	robot.DFICycleCount = fToI(r.DFICycleCount)
	robot.DFITriggered = r.DFITriggered
	robot.LitterRobotID = r.LitterRobotID
	robot.LitterRobotSerial = r.LitterRobotSerial
	robot.Name = r.Name
	robot.NightLightActive = r.NightLightActive
	robot.PanelLockActive = r.PanelLockActive
	robot.PowerStatus = r.PowerStatus
	robot.SleepModeActive = r.SleepModeActive
	robot.UnitStatus = fToI(r.UnitStatus)

	return robot

}

func fToI(f float64) (i int) {
	if math.IsNaN(f) {
		i = -1
	} else {
		i = int(f)
	}
	return i
}
