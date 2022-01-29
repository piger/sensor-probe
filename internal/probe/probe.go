package probe

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/brutella/hc/accessory"
	"github.com/piger/sensor-probe/internal/config"
	"github.com/piger/sensor-probe/internal/homekit"
	"github.com/piger/sensor-probe/internal/sensors"
	"github.com/piger/sensor-probe/internal/sensors/mijia"
	"github.com/piger/sensor-probe/internal/sensors/ruuvi"
	"gitlab.com/jtaimisto/bluewalker/filter"
	"gitlab.com/jtaimisto/bluewalker/hci"
	"gitlab.com/jtaimisto/bluewalker/host"
)

// Probe is the structure that holds the state of this program.
type Probe struct {
	device string
	config *config.Config
}

// New creates a new Probe object.
func New(device string, config *config.Config) *Probe {
	return &Probe{device: device, config: config}
}

func initRadio(device string) (*host.Host, error) {
	raw, err := hci.Raw(device)
	if err != nil {
		return nil, fmt.Errorf("opening bluetooth device %q: %w", device, err)
	}

	// Initialize initialises the Bluetooth device; please note that the bluetooth device must be "off"
	// when this function is called.
	hostRadio := host.New(raw)
	if err := hostRadio.Init(); err != nil {
		return nil, fmt.Errorf("initializing bluetooth engine (try hciconfig <device> down): %w", err)
	}

	return hostRadio, nil
}

// Run is this program's main loop.
func (p *Probe) Run() error {
	hostRadio, err := initRadio(p.device)
	if err != nil {
		return err
	}
	defer hostRadio.Deinit()

	ctx := context.Background()
	defer ctx.Done()

	sensorsDB := make(map[string]sensors.SensorUpdater)
	var hkAccs []*accessory.Accessory

	// IMPORTANT: sensors must always be added in the same order!
	for i, sensorConfig := range p.config.Sensors {
		id := uint64(i + 2)
		log.Printf("adding sensor %s (%s) with ID %d", sensorConfig.Name, sensorConfig.MAC, id)

		var sensor sensors.SensorUpdater
		if sensorConfig.Firmware == "custom" {
			sensor = mijia.NewMijiaSensor(&sensorConfig, id)
		} else if sensorConfig.Firmware == "ruuviv5" {
			sensor = ruuvi.NewRuuviSensor(&sensorConfig, id)
		} else {
			return fmt.Errorf("sensor of type %s is unsupported", sensorConfig.Firmware)
		}

		sensorsDB[sensorConfig.MAC] = sensor
		hkAccs = append(hkAccs, sensor.GetAccessory().Accessory)
	}

	hkTransport, err := homekit.SetupHomeKit(&p.config.HomeKit, hkAccs)
	if err != nil {
		return err
	}
	homekit.PrintQRcode(hkTransport, os.Stdout)

	log.Println("starting HomeKit subsystem")
	go hkTransport.Start()

	filters, err := buildFilters(p.config.Sensors)
	if err != nil {
		return fmt.Errorf("building filters: %s", err)
	}

	log.Println("starting Bluetooth scanner")
	reportChan, err := hostRadio.StartScanning(false, filters)
	if err != nil {
		return fmt.Errorf("starting scan: %w", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case report := <-reportChan:
			addr := strings.ToUpper(report.Address.String())
			if sensor, ok := sensorsDB[addr]; ok {
				if err := sensor.Update(ctx, report, p.config.DBConfig); err != nil {
					log.Print(err)
				}
			} else {
				log.Printf("can't lookup sensor %s internally, this is probably a bug", addr)
			}

		case sig := <-sigs:
			log.Printf("signal received: %s", sig)

			log.Print("stopping bluetooth scan")
			if err := hostRadio.StopScanning(); err != nil {
				log.Printf("error stopping scan: %s", err)
			}

			log.Print("stopping homekit subsystem")
			<-hkTransport.Stop()

			return nil
		}
	}
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
