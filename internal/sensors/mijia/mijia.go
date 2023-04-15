// Xiaomi Thermometer LYWSD03MMC
// https://github.com/atc1441/ATC_MiThermometer

package mijia

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/brutella/hc/accessory"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/piger/sensor-probe/internal/config"
	"github.com/piger/sensor-probe/internal/db"
	"github.com/piger/sensor-probe/internal/homekit"
	"github.com/piger/sensor-probe/internal/sensors"
	"gitlab.com/jtaimisto/bluewalker/hci"
	"gitlab.com/jtaimisto/bluewalker/host"
)

// Manufacturer ID for the custom firmware; see also:
// https://github.com/atc1441/ATC_MiThermometer#advertising-format-of-the-custom-firmware
const UUID = 0x181a

type payload struct {
	UUID         uint16
	MAC          [6]uint8
	Temperature  int16
	Humidity     uint8
	Battery      uint8
	BatterymVolt uint16
	FrameCounter uint8
}

type Data struct {
	Temperature float32
	Humidity    float32
	Battery     uint16
	BatteryVolt float32
}

func parseMessage(b []byte) (*Data, error) {
	var p payload
	buf := bytes.NewBuffer(b)
	if err := binary.Read(buf, binary.BigEndian, &p); err != nil {
		return nil, err
	}

	data := Data{
		Temperature: float32(p.Temperature) / 10.0,
		Humidity:    float32(p.Humidity),
		Battery:     uint16(p.Battery),
		BatteryVolt: float32(p.BatterymVolt),
	}
	return &data, nil
}

func checkReport(r *hci.AdStructure) bool {
	return r.Typ == hci.AdServiceData && len(r.Data) >= 2 && binary.LittleEndian.Uint16(r.Data) == UUID
}

type MijiaSensor struct {
	*sensors.Sensor
	LastData *Data
}

func NewMijiaSensor(config *config.SensorConfig, id uint64) *MijiaSensor {
	info := accessory.Info{
		Name:         config.Name,
		Model:        "Xiaomi Thermometer LYWSD03MMC",
		SerialNumber: "ABCDEFG",
		Manufacturer: "Xiaomi",
		ID:           id,
	}

	acc := homekit.NewTemperatureHumiditySensor(info)
	s := sensors.NewSensor(config, acc)

	ms := MijiaSensor{
		Sensor:   s,
		LastData: nil,
	}
	return &ms
}

func (m *MijiaSensor) GetName() string {
	return m.Name
}

func (m *MijiaSensor) Update(report *host.ScanReport) error {
	for _, ads := range report.Data {
		if checkReport(ads) {
			if err := m.handleBroadcast(ads); err != nil {
				log.Print(err)
			}
		}
	}
	return nil
}

func (m *MijiaSensor) handleBroadcast(msg *hci.AdStructure) error {
	data, err := parseMessage(msg.Data)
	if err != nil {
		return err
	}
	m.LastData = data

	now := time.Now()
	// log.Printf("%q (%s): T=%.2f H=%.2f%% B=%d%%", m.Name, m.MAC, data.Temperature, data.Humidity, data.Battery)

	if m.LastUpdateHomeKit.IsZero() || now.Sub(m.LastUpdateHomeKit) >= sensors.HomeKitUpdateInterval {
		m.SetTemperature(float64(data.Temperature))
		m.SetHumidity(float64(data.Humidity))
		m.LastUpdateHomeKit = now
	}

	return nil
}

var columnNames = []string{
	"time",
	"room",
	"temperature",
	"humidity",
	"battery",
}

func (m *MijiaSensor) Push(ctx context.Context, pool *pgxpool.Pool, t time.Time) error {
	if m.LastData == nil {
		return errors.New("no last data")
	}

	ctx, cancel := context.WithTimeout(ctx, db.DBConnTimeout)
	defer cancel()

	columns := db.MakeColumnString(columnNames)
	values := db.MakeValuesString(columnNames)

	if _, err := pool.Exec(ctx,
		fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", m.DBTable, columns, values),
		t,
		m.Name,
		m.LastData.Temperature,
		m.LastData.Humidity,
		m.LastData.Battery,
	); err != nil {
		return fmt.Errorf("error writing row to DB: %w", err)
	}

	return nil
}
