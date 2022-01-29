package sensors

import (
	"context"
	"time"

	"github.com/piger/sensor-probe/internal/config"
	"github.com/piger/sensor-probe/internal/homekit"
	"gitlab.com/jtaimisto/bluewalker/host"
)

const (
	UpdateDelayHk = 2 * time.Minute
	UpdateDelayDB = 5 * time.Minute
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
	// data and DB config
	Update(context.Context, *host.ScanReport, string) error
	GetAccessory() *homekit.TemperatureHumiditySensor
}
