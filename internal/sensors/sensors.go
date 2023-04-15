package sensors

import (
	"context"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/piger/sensor-probe/internal/config"
	"github.com/piger/sensor-probe/internal/homekit"
	"gitlab.com/jtaimisto/bluewalker/host"
)

const (
	HomeKitUpdateInterval  = 2 * time.Minute
	DatabaseUpdateInterval = 5 * time.Minute
)

type Sensor struct {
	Name              string
	MAC               string
	DBTable           string
	Firmware          string
	Accessory         *homekit.TemperatureHumiditySensor
	LastUpdateHomeKit time.Time
	LastUpdateDB      time.Time
}

func NewSensor(config *config.SensorConfig, acc *homekit.TemperatureHumiditySensor) *Sensor {
	s := Sensor{
		Name:      config.Name,
		MAC:       config.MAC,
		DBTable:   config.DBTable,
		Firmware:  config.Firmware,
		Accessory: acc,
	}
	return &s
}

func (s *Sensor) SetTemperature(v float64) {
	s.Accessory.TemperatureSensor.CurrentTemperature.SetValue(v)
}

func (s *Sensor) SetHumidity(v float64) {
	s.Accessory.HumiditySensor.CurrentRelativeHumidity.SetValue(v)
}

func (s *Sensor) GetAccessory() *homekit.TemperatureHumiditySensor {
	return s.Accessory
}

type SensorUpdater interface {
	// GetName returns the name of a Sensor; it's used to name the sensor in error messages.
	GetName() string

	// Update search for sensor data in a bluetooth broadcast, set them in HomeKit and store the data.
	Update(*host.ScanReport) error

	// Push push the latest set of sensor data to the metrics server.
	Push(context.Context, *pgxpool.Pool, time.Time) error

	// GetAccessory returns the embedded HomeKit accessory.
	GetAccessory() *homekit.TemperatureHumiditySensor
}
