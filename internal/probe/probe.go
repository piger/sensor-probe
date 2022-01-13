package probe

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	hcLog "github.com/brutella/hc/log"
	"github.com/mdp/qrterminal/v3"
	"github.com/piger/sensor-probe/internal/config"
	"gitlab.com/jtaimisto/bluewalker/filter"
	"gitlab.com/jtaimisto/bluewalker/hci"
	"gitlab.com/jtaimisto/bluewalker/host"
)

type Probe struct {
	device    string
	config    *config.Config
	hostRadio *host.Host
}

type sensorData struct {
	UUID         uint16
	MAC          [6]uint8
	Temperature  int16
	Humidity     uint8
	Battery      uint8
	BatterymVolt uint16
	FrameCounter uint8
}

type SensorStatus struct {
	Temperature float64
	Humidity    int
	Battery     int
	BatteryVolt int
}

func New(device string, config *config.Config) *Probe {
	p := Probe{
		device: device,
		config: config,
	}

	return &p
}

func (p *Probe) Initialize() error {
	raw, err := hci.Raw(p.device)
	if err != nil {
		return fmt.Errorf("opening bluetooth device %q: %w", p.device, err)
	}

	hostRadio := host.New(raw)
	if err := hostRadio.Init(); err != nil {
		return fmt.Errorf("initializing bluetooth engine: %w", err)
	}
	p.hostRadio = hostRadio

	// NOTE: must call host.Deinit() at the end!

	return nil
}

func (p *Probe) Run() error {
	if p.hostRadio == nil {
		if err := p.Initialize(); err != nil {
			return err
		}
	}
	defer p.hostRadio.Deinit()

	doActiveScan := false
	filters, err := buildFilters(p.config.Sensors)
	if err != nil {
		return fmt.Errorf("building filters: %s", err)
	}

	nameMap := make(map[string]string)
	for _, sensor := range p.config.Sensors {
		nameMap[sensor.MAC] = sensor.Name
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	reportChan, err := p.hostRadio.StartScanning(doActiveScan, filters)
	if err != nil {
		return fmt.Errorf("starting scan: %w", err)
	}

	currentValues := make(map[string]SensorStatus)
	ticker := time.NewTicker(p.config.Interval.Duration)
	defer ticker.Stop()

	ctx := context.Background()
	defer ctx.Done()

	// initialise homekit
	hcLog.Debug.Enable()
	hkBridge := accessory.NewBridge(accessory.Info{
		Name:         "Sensor Probe",
		Manufacturer: "Kertesz Industries",
		SerialNumber: "100",
		Model:        "ABBESTIA",
	})

	hkSensors := make(map[string]*accessory.Thermometer)
	for _, sensor := range p.config.Sensors {
		hkSensors[sensor.MAC] = accessory.NewTemperatureSensor(accessory.Info{
			Name:         sensor.Name,
			Model:        "Xiaomi Thermometer LYWSD03MMC",
			SerialNumber: "ABCDEFG",
			Manufacturer: "Xiaomi",
		}, 0, 0, 50, 1)
	}

	var hkAccs []*accessory.Accessory
	for _, a := range hkSensors {
		log.Printf("adding HomeKit sensor: %+v\n", a)
		a.TempSensor.CurrentTemperature.SetValue(10)
		hkAccs = append(hkAccs, a.Accessory)
	}

	hkConfig := hc.Config{
		Pin:     p.config.HomeKit.Pin,
		SetupId: p.config.HomeKit.SetupID,
		Port:    strconv.Itoa(p.config.HomeKit.Port),
	}
	hkTransport, err := hc.NewIPTransport(hkConfig, hkBridge.Accessory, hkAccs...)
	if err != nil {
		return fmt.Errorf("initializing homekit: %w", err)
	}

	uri, err := hkTransport.XHMURI()
	if err != nil {
		return fmt.Errorf("error getting XHM URI: %w", err)
	}
	qrterminal.Generate(uri, qrterminal.L, os.Stdout)

	go hkTransport.Start()

Loop:
	for {
		select {
		case report := <-reportChan:
			sd, addr, found, err := parseReport(report)
			switch {
			case err != nil:
				log.Printf("error: %s", err)
				continue
			case !found:
				continue
			}

			var name string
			if n, ok := nameMap[addr]; ok {
				name = n
			} else {
				name = addr
			}

			temperature := float64(sd.Temperature) / 10.0

			log.Printf("%q (%s): T=%.2f H=%d%% B=%d%%", name, addr, temperature, sd.Humidity, sd.Battery)

			// Store current values for this sensor
			currentValues[name] = SensorStatus{
				Temperature: temperature,
				Humidity:    int(sd.Humidity),
				Battery:     int(sd.Battery),
				BatteryVolt: int(sd.BatterymVolt),
			}

			// update temperature in HomeKit
			if hks, ok := hkSensors[addr]; ok {
				hks.TempSensor.CurrentTemperature.SetValue(temperature)
			}

			// update Prometheus metrics
			temperatureMetric.WithLabelValues(name, addr).Set(temperature)
			humidityMetric.WithLabelValues(name, addr).Set(float64(sd.Humidity))
			batteryMetric.WithLabelValues(name, addr).Set(float64(sd.Battery))

		case t := <-ticker.C:
			log.Print("sending current status to DB")
			now := t.UTC()

			for name, status := range currentValues {
				fmt.Printf("name=%s temperature=%.2f humidity=%d%% battery=%d batteryV=%d\n",
					name, status.Temperature, status.Humidity, status.Battery, status.BatteryVolt)

				if err := writeDBRow(ctx, now, name, status, p.config.DBConfig, "home_temperature"); err != nil {
					log.Printf("error writing DB row: %s", err)
				}
			}

		case sig := <-sigs:
			log.Printf("signal received: %s", sig)

			log.Print("stopping bluetooth scan")
			if err := p.hostRadio.StopScanning(); err != nil {
				log.Printf("error stopping scan: %s", err)
			}

			log.Print("stopping homekit server")
			<-hkTransport.Stop()

			break Loop
		}
	}

	return nil
}

func buildFilters(sensors []config.SensorConfig) ([]filter.AdFilter, error) {
	addrFilters := make([]filter.AdFilter, len(sensors))
	for i, sensor := range sensors {
		baddr, err := hci.BtAddressFromString(sensor.MAC)
		if err != nil {
			return nil, fmt.Errorf("parsing MAC address %q: %w", sensor.MAC, err)
		}
		addrFilters[i] = filter.ByAddress(baddr)
	}

	filters := []filter.AdFilter{
		filter.Any(addrFilters),
	}

	return filters, nil
}

func getServiceData(report *host.ScanReport) *hci.AdStructure {
	for _, reportData := range report.Data {
		if reportData.Typ == hci.AdServiceData {
			return reportData
		}
	}

	return nil
}

func parseReport(report *host.ScanReport) (*sensorData, string, bool, error) {
	serviceData := getServiceData(report)
	if serviceData == nil {
		return nil, "", false, nil
	}

	buf := bytes.NewBuffer(serviceData.Data)
	var sd sensorData
	if err := binary.Read(buf, binary.BigEndian, &sd); err != nil {
		return nil, "", false, fmt.Errorf("error parsing sensor data from %s: %w", report.Address, err)
	}

	addr := strings.ToUpper(report.Address.String())
	return &sd, addr, true, nil
}
