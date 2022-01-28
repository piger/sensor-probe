package probe

import "time"

type Sensor struct {
	Name              string
	MAC               string
	DBTable           string
	Accessory         *TemperatureHumiditySensor
	LastUpdateHomeKit time.Time
	LastUpdateDB      time.Time
}

func NewSensor(name, address, table string, id uint64) *Sensor {
	s := Sensor{
		Name:      name,
		MAC:       address,
		DBTable:   table,
		Accessory: NewMijiaSensor(name, id),
	}

	return &s
}
