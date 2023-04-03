package homekit

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/service"
	"github.com/piger/sensor-probe/internal/config"
	"rsc.io/qr"
)

var (
	//go:embed webpage.html
	webpage     string
	webTemplate *template.Template
)

func init() {
	webTemplate = template.Must(template.New("").Parse(webpage))
}

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

// StartHttpServer starts an HTTP server meant to only serve the setup QR code to the user.
func StartHttpServer(listener net.Listener, hkURI string) error {
	server := http.Server{
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code, err := qr.Encode(hkURI, qr.H)
		if err != nil {
			log.Printf("error generating QR code: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := webTemplate.Execute(w, base64.StdEncoding.EncodeToString(code.PNG())); err != nil {
			log.Printf("error serving root page: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	return server.Serve(listener)
}
