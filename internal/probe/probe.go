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
	"github.com/mdp/qrterminal/v3"
	"github.com/piger/sensor-probe/internal/config"
	"github.com/piger/sensor-probe/internal/sensors/mijia"
	"github.com/piger/sensor-probe/internal/sensors/ruuvi"
	"gitlab.com/jtaimisto/bluewalker/filter"
	"gitlab.com/jtaimisto/bluewalker/hci"
	"gitlab.com/jtaimisto/bluewalker/host"
)

// Probe is the structure that holds the state of this program.
type Probe struct {
	device    string
	config    *config.Config
	hostRadio *host.Host
}

// sensorData represent a "status update" sent by a temperature sensor via BLE broadcasts.
type sensorData struct {
	UUID         uint16
	MAC          [6]uint8
	Temperature  int16
	Humidity     uint8
	Battery      uint8
	BatterymVolt uint16
	FrameCounter uint8
}

// SensorStatus is used to keep track of each sensor's current status. This data will be periodically
// sent to the database.
type SensorStatus struct {
	Temperature float64
	Humidity    int
	Battery     int
	BatteryVolt int
}

type Sensor struct {
	Name      string
	Address   string
	Accessory *accessory.Thermometer

	status *SensorStatus
}

func NewSensor(name, address string, id uint64) *Sensor {
	info := accessory.Info{
		Name:         name,
		Model:        "Xiaomi Thermometer LYWSD03MMC",
		SerialNumber: "ABCDEFG",
		Manufacturer: "Xiaomi",
		ID:           id,
	}
	acc := accessory.NewTemperatureSensor(info, 0, 0, 50, 1)
	s := Sensor{
		Name:      name,
		Address:   address,
		Accessory: acc,
	}

	return &s
}

func (s *Sensor) GetStatus() (*SensorStatus, bool) {
	if s.status != nil {
		return s.status, true
	}
	return nil, false
}

func (s *Sensor) SetStatus(temperature float64, humidity, battery, batteryVolt int) {
	status := SensorStatus{
		Temperature: temperature,
		Humidity:    humidity,
		Battery:     battery,
		BatteryVolt: batteryVolt,
	}
	s.status = &status
}

// New creates a new Probe object; it doesn't initialise the bluetooth device, which must be explicitly
// initialised by calling p.Initialize().
func New(device string, config *config.Config) *Probe {
	p := Probe{
		device: device,
		config: config,
	}

	return &p
}

// Initialize initialises the Bluetooth device; please note that the bluetooth device must be "off"
// when this function is called.
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

// Run is this program's main loop.
func (p *Probe) Run() error {
	if p.hostRadio == nil {
		if err := p.Initialize(); err != nil {
			return err
		}
	}
	defer p.hostRadio.Deinit()

	// Setup internal maps and channels
	sensorsDB := make(map[string]*Sensor)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(p.config.Interval.Duration)
	defer ticker.Stop()

	ctx := context.Background()
	defer ctx.Done()

	// initialise homekit
	hkBridge := accessory.NewBridge(accessory.Info{
		Name:         "Sensor Probe",
		Manufacturer: "Kertesz Industries",
		SerialNumber: "100",
		Model:        "ABBESTIA",
		ID:           1,
	})

	var hkAccs []*accessory.Accessory

	// IMPORTANT: sensors must always be added in the same order!
	for i, sensorConfig := range p.config.Sensors {
		id := uint64(i + 2)
		log.Printf("adding sensor %s (%s) with ID %d", sensorConfig.Name, sensorConfig.MAC, id)

		sensor := NewSensor(sensorConfig.Name, sensorConfig.MAC, id)
		sensorsDB[sensorConfig.MAC] = sensor
		hkAccs = append(hkAccs, sensor.Accessory.Accessory)
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

	// Print QR code
	uri, err := hkTransport.XHMURI()
	if err != nil {
		return fmt.Errorf("error getting XHM URI: %w", err)
	}
	qrterminal.Generate(uri, qrterminal.L, os.Stdout)

	// Start HomeKit subsystem
	go hkTransport.Start()

	// Start Bluetooth scan
	filters, err := buildFilters(p.config.Sensors)
	if err != nil {
		return fmt.Errorf("building filters: %s", err)
	}
	reportChan, err := p.hostRadio.StartScanning(false, filters)
	if err != nil {
		return fmt.Errorf("starting scan: %w", err)
	}

Loop:
	for {
		select {
		case report := <-reportChan:
			sd, addr, found, err := parseReport(report)
			switch {
			case err != nil:
				log.Printf("error parsing data from %s: %s", addr, err)
				continue
			case !found:
				continue
			}

			sensor, ok := sensorsDB[addr]
			if !ok {
				log.Printf("can't lookup sensor %s internally, this is probably a bug", addr)
				continue
			}

			temperature := float64(sd.Temperature) / 10.0

			log.Printf("%q (%s): T=%.2f H=%d%% B=%d%%", sensor.Name, addr, temperature, sd.Humidity, sd.Battery)

			// Store current values for this sensor
			sensor.SetStatus(temperature, int(sd.Humidity), int(sd.Battery), int(sd.BatterymVolt))

			// update temperature in HomeKit
			sensor.Accessory.TempSensor.CurrentTemperature.SetValue(temperature)

		case t := <-ticker.C:
			log.Print("sending current status to DB")
			now := t.UTC()

			for _, sensor := range sensorsDB {
				if status, ok := sensor.GetStatus(); ok {
					if err := writeDBRow(ctx, now, sensor.Name, status, p.config.DBConfig, "home_temperature"); err != nil {
						log.Printf("error writing DB row: %s", err)
					}
				}
			}

		case sig := <-sigs:
			log.Printf("signal received: %s", sig)

			log.Print("stopping bluetooth scan")
			if err := p.hostRadio.StopScanning(); err != nil {
				log.Printf("error stopping scan: %s", err)
			}

			log.Print("stopping homekit subsystem")
			<-hkTransport.Stop()

			break Loop
		}
	}

	return nil
}

// buildFilters builds a filter set for bluewalker to only capture events sent from devices
// having the specified MAC addresses.
func buildFilters(sensors []config.SensorConfig) ([]filter.AdFilter, error) {
	addrFilters := make([]filter.AdFilter, len(sensors))
	for i, sensor := range sensors {
		baddr, err := hci.BtAddressFromString(sensor.MAC)
		if err != nil {
			return nil, fmt.Errorf("parsing MAC address %q: %w", sensor.MAC, err)
		}

		// hack to support BT random addresses
		if sensor.Firmware == "ruuviv5" {
			baddr.Atype = hci.LeRandomAddress
		}
		addrFilters[i] = filter.ByAddress(baddr)
	}

	filters := []filter.AdFilter{
		filter.Any(addrFilters),
		/*
			filter.Any([]filter.AdFilter{
				filter.ByVendor([]byte{0x99, 0x04}),
				filter.ByAdData(hci.AdServiceData, []byte{0x1a, 0x18}),
			}),
		*/
	}

	return filters, nil
}

// getServiceData read a bluewalker report and returns a "Service Data" object
// if found and nil otherwise.
func getServiceData(report *host.ScanReport) *hci.AdStructure {
	for _, reportData := range report.Data {
		if reportData.Typ == hci.AdServiceData {
			return reportData
		}
	}

	return nil
}

// parseReport parse a scan report from bluewalker and extracts a sensor's status data
// and returns a sensorData object, the MAC address of the device, a boolean indicating wether any
// data was found and an error.
func parseReport(report *host.ScanReport) (*sensorData, string, bool, error) {
	for _, rd := range report.Data {
		if rd.Typ == hci.AdServiceData && len(rd.Data) >= 2 && binary.LittleEndian.Uint16(rd.Data) == 0x181a {
			log.Println("broadcast from xiaomi mijia")

			data, err := mijia.ParseMessage(rd)
			if err != nil {
				log.Printf("parse mijia err: %s", err)
			} else {
				log.Printf("mijia data: %+v", data)
			}
		} else if rd.Typ == hci.AdManufacturerSpecific && len(rd.Data) >= 2 && binary.LittleEndian.Uint16(rd.Data) == 0x0499 {
			// https://github.com/ruuvi/ruuvi-sensor-protocols/blob/master/dataformat_05.md
			log.Println("broadcast from ruuvi")

			data, err := ruuvi.ParseMessage(rd)
			if err != nil {
				log.Printf("parse ruuvi err: %s", err)
			} else {
				log.Printf("ruuvi data: %+v", data)
			}

			// exit early because our parser isn't ready!
			return nil, "", false, nil
		}
	}

	serviceData := getServiceData(report)
	if serviceData == nil {
		return nil, "", false, nil
	}

	addr := strings.ToUpper(report.Address.String())

	buf := bytes.NewBuffer(serviceData.Data)
	var sd sensorData
	if err := binary.Read(buf, binary.BigEndian, &sd); err != nil {
		return nil, addr, false, err
	}

	return &sd, addr, true, nil
}
