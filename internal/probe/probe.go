package probe

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/brutella/hc/accessory"
	"github.com/jackc/pgx/v4/pgxpool"
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

	ctx, stopCtx := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	pool, err := pgxpool.Connect(ctx, p.config.DBConfig)
	if err != nil {
		return err
	}
	defer pool.Close()

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

	// Start an HTTP server on a random free port, to show the HomeKit setup QR code.
	homeKitURI, err := hkTransport.XHMURI()
	if err != nil {
		return err
	}

	httpListener, err := net.Listen("tcp", ":0")
	if err != nil {
		return err
	}
	log.Printf("starting HTTP server on port %d", httpListener.Addr().(*net.TCPAddr).Port)

	go func() {
		if err := homekit.StartHttpServer(httpListener, homeKitURI); err != nil {
			log.Printf("error from HTTP server: %s", err)
		}
	}()

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

	tick := time.NewTicker(sensors.DatabaseUpdateInterval)
	defer tick.Stop()

Loop:
	for {
		select {
		case report := <-reportChan:
			addr := strings.ToUpper(report.Address.String())
			if sensor, ok := sensorsDB[addr]; ok {
				if err := sensor.Update(report); err != nil {
					log.Print(err)
				}
			} else {
				log.Printf("can't lookup sensor %s internally, this is probably a bug", addr)
			}

		case ts := <-tick.C:
			for _, sensor := range sensorsDB {
				if err := sensor.Push(ctx, pool, ts); err != nil {
					log.Printf("error sending metrics from %s: %s", sensor.GetName(), err)
				}
			}

		case <-ctx.Done():
			log.Printf("signal received (%v); starting shutdown", ctx.Err())
			// we call stop() on the context here, so that further interrupt signals will
			// forcefully terminate the program.
			stopCtx()

			log.Print("stopping bluetooth scan")
			if err := hostRadio.StopScanning(); err != nil {
				log.Printf("error stopping scan: %s", err)
			}

			log.Print("stopping homekit subsystem")
			select {
			case <-time.After(20 * time.Second):
				log.Print("timeout while waiting for homekit subsystem to stop")
				break Loop
			case <-hkTransport.Stop():
				break Loop
			}
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
