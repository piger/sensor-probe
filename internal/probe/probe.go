package probe

import (
	"context"
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
	"gitlab.com/jtaimisto/bluewalker/filter"
	"gitlab.com/jtaimisto/bluewalker/hci"
	"gitlab.com/jtaimisto/bluewalker/host"
)

// Probe is the structure that holds the state of this program.
type Probe struct {
	device string
	config *config.Config
}

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

// New creates a new Probe object; it doesn't initialise the bluetooth device, which must be explicitly
// initialised by calling p.Initialize().
func New(device string, config *config.Config) *Probe {
	p := Probe{
		device: device,
		config: config,
	}

	return &p
}

// Run is this program's main loop.
func (p *Probe) Run() error {
	raw, err := hci.Raw(p.device)
	if err != nil {
		return fmt.Errorf("opening bluetooth device %q: %w", p.device, err)
	}

	// Initialize initialises the Bluetooth device; please note that the bluetooth device must be "off"
	// when this function is called.
	hostRadio := host.New(raw)
	if err := hostRadio.Init(); err != nil {
		return fmt.Errorf("initializing bluetooth engine: %w", err)
	}
	defer hostRadio.Deinit()

	// Setup internal maps and channels
	sensorsDB := make(map[string]*Sensor)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

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
	reportChan, err := hostRadio.StartScanning(false, filters)
	if err != nil {
		return fmt.Errorf("starting scan: %w", err)
	}

Loop:
	for {
		select {
		case report := <-reportChan:
			data, addr, found, err := parseMijiaReport(report)

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

			log.Printf("%q (%s): T=%.2f H=%.2f%% B=%d%%", sensor.Name, addr, data.Temperature, data.Humidity, data.Battery)

			now := time.Now()

			if sensor.LastUpdateHomeKit.IsZero() || now.Sub(sensor.LastUpdateHomeKit) >= 2*time.Minute {
				log.Printf("Updating HomeKit for %s", addr)
				sensor.Accessory.SetTemperature(float64(data.Temperature))
				sensor.Accessory.SetHumidity(float64(data.Humidity))
				sensor.LastUpdateHomeKit = now
			}

			if sensor.LastUpdateDB.IsZero() || now.Sub(sensor.LastUpdateDB) >= 2*time.Minute {
				log.Printf("Updating DB for %s", addr)
				if err := writeDBRow(ctx, now, sensor.Name, data, p.config.DBConfig, "home_temperature"); err != nil {
					log.Printf("error writing DB row: %s", err)
				}
				sensor.LastUpdateDB = now
			}

		case sig := <-sigs:
			log.Printf("signal received: %s", sig)

			log.Print("stopping bluetooth scan")
			if err := hostRadio.StopScanning(); err != nil {
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
		filter.Any([]filter.AdFilter{
			filter.ByVendor([]byte{0x99, 0x04}),
			filter.ByAdData(hci.AdServiceData, []byte{0x1a, 0x18}),
		}),
	}

	return filters, nil
}

func parseMijiaReport(report *host.ScanReport) (*mijia.Data, string, bool, error) {
	addr := strings.ToUpper(report.Address.String())

	for _, ads := range report.Data {
		if mijia.CheckReport(ads) {
			data, err := mijia.ParseMessage(ads)
			if err != nil {
				return nil, addr, false, err
			}
			return data, addr, true, err
		}
	}

	return nil, addr, false, nil
}
