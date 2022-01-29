package homekit

import (
	"fmt"
	"io"
	"log"
	"strconv"

	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/service"
	"github.com/mdp/qrterminal/v3"
	"github.com/piger/sensor-probe/internal/config"
)

type HomeKitTransport interface {
	// Start starts the transport
	Start()

	// Stop stops the transport
	// Use the returned channel to wait until the transport is fully stopped.
	Stop() <-chan struct{}

	// XHMURI returns a X-HM styled uri to easily add the accessory to HomeKit.
	XHMURI() (string, error)
}

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

func SetupHomeKit(config *config.HomeKit, accs []*accessory.Accessory) (HomeKitTransport, error) {
	hkBridge := accessory.NewBridge(accessory.Info{
		Name:         "Sensor Probe",
		Manufacturer: "Kertesz Industries",
		SerialNumber: "100",
		Model:        "ABBESTIA",
		ID:           1,
	})

	hkConfig := hc.Config{
		Pin:     config.Pin,
		SetupId: config.SetupID,
		Port:    strconv.Itoa(config.Port),
	}
	hkTransport, err := hc.NewIPTransport(hkConfig, hkBridge.Accessory, accs...)
	if err != nil {
		return nil, fmt.Errorf("initializing homekit: %w", err)
	}

	return hkTransport, nil
}

func PrintQRcode(t HomeKitTransport, w io.Writer) {
	uri, err := t.XHMURI()
	if err != nil {
		log.Printf("error getting XHM URI: %s", err)
		return
	}
	qrterminal.Generate(uri, qrterminal.L, w)
}
