package probe

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	reportChan, err := p.hostRadio.StartScanning(doActiveScan, filters)
	if err != nil {
		return fmt.Errorf("starting scan: %w", err)
	}

Loop:
	for {
		select {
		case report := <-reportChan:
			for _, reportData := range report.Data {
				if reportData.Typ == hci.AdServiceData {
					buf := bytes.NewBuffer(reportData.Data)
					var sd sensorData
					if err := binary.Read(buf, binary.BigEndian, &sd); err != nil {
						log.Printf("error parsing sensor data from %s: %s", report.Address, err)
						continue // XXX
					}

					fmt.Printf("%s: T=%.2f H=%d%% B=%d%%\n", report.Address, float32(sd.Temperature)/10.0, sd.Humidity, sd.Battery)
				}
			}
		case sig := <-sigs:
			log.Printf("signal received: %s", sig)
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
