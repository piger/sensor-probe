package probe

import "time"

type Sensor struct {
	Name              string
	Address           string
	Accessory         *TemperatureHumiditySensor
	LastUpdateHomeKit time.Time
	LastUpdateDB      time.Time
}

func NewSensor(name, address string, id uint64) *Sensor {
	s := Sensor{
		Name:      name,
		Address:   address,
		Accessory: NewMijiaSensor(name, id),
	}

	return &s
}
