package probe

import (
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/service"
)

type TemperatureHumiditySensor struct {
	*accessory.Accessory
	TemperatureSensor *service.TemperatureSensor
	HumiditySensor    *service.HumiditySensor
}

func NewTemperatureHumiditySensor(info accessory.Info) *TemperatureHumiditySensor {
	acc := TemperatureHumiditySensor{}
	acc.Accessory = accessory.New(info, accessory.TypeThermostat)

	acc.TemperatureSensor = service.NewTemperatureSensor()
	acc.AddService(acc.TemperatureSensor.Service)

	acc.HumiditySensor = service.NewHumiditySensor()
	acc.AddService(acc.HumiditySensor.Service)

	return &acc
}

func NewMijiaSensor(name string, id uint64) *TemperatureHumiditySensor {
	info := accessory.Info{
		Name:         name,
		Model:        "Xiaomi Thermometer LYWSD03MMC",
		SerialNumber: "ABCDEFG",
		Manufacturer: "Xiaomi",
		ID:           id,
	}

	s := NewTemperatureHumiditySensor(info)
	return s
}

func (s *TemperatureHumiditySensor) SetTemperature(v float64) {
	s.TemperatureSensor.CurrentTemperature.SetValue(v)
}

func (s *TemperatureHumiditySensor) SetHumidity(v float64) {
	s.HumiditySensor.CurrentRelativeHumidity.SetValue(v)
}
