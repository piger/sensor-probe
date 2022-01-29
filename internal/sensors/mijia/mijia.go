// Xiaomi Thermometer LYWSD03MMC
// https://github.com/atc1441/ATC_MiThermometer

package mijia

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"github.com/brutella/hc/accessory"
	"github.com/jackc/pgx/v4"
	"github.com/piger/sensor-probe/internal/config"
	"github.com/piger/sensor-probe/internal/db"
	"github.com/piger/sensor-probe/internal/homekit"
	"github.com/piger/sensor-probe/internal/sensors"
	"gitlab.com/jtaimisto/bluewalker/hci"
	"gitlab.com/jtaimisto/bluewalker/host"
)

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

func parseMessage(ads *hci.AdStructure) (*Data, error) {
	var p payload
	buf := bytes.NewBuffer(ads.Data)
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

const UUID = 0x181a

func checkReport(r *hci.AdStructure) bool {
	return r.Typ == hci.AdServiceData && len(r.Data) >= 2 && binary.LittleEndian.Uint16(r.Data) == UUID
}

type MijiaSensor struct {
	*sensors.Sensor
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

	return &MijiaSensor{s}
}

func (m *MijiaSensor) Update(report *host.ScanReport, dbconfig string) error {
	for _, ads := range report.Data {
		if checkReport(ads) {
			if err := m.handleBroadcast(ads, dbconfig); err != nil {
				log.Print(err)
			}
		}
	}
	return nil
}

func (m *MijiaSensor) handleBroadcast(b *hci.AdStructure, dbconfig string) error {
	data, err := parseMessage(b)
	if err != nil {
		return err
	}

	now := time.Now()
	log.Printf("%q (%s): T=%.2f H=%.2f%% B=%d%%", m.Name, m.MAC, data.Temperature, data.Humidity, data.Battery)

	if m.LastUpdateHomeKit.IsZero() || now.Sub(m.LastUpdateHomeKit) >= sensors.UpdateDelayHk {
		m.SetTemperature(float64(data.Temperature))
		m.SetHumidity(float64(data.Humidity))
		m.LastUpdateHomeKit = now
	}

	ctx := context.TODO()
	if m.LastUpdateDB.IsZero() || now.Sub(m.LastUpdateDB) >= sensors.UpdateDelayDB {
		if err := writeDBRow(ctx, now, m.Name, data, dbconfig, m.DBTable); err != nil {
			return err
		}
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

func writeDBRow(ctx context.Context, t time.Time, room string, data *Data, dburl, table string) error {
	ctx, cancel := context.WithTimeout(ctx, db.DBConnTimeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, dburl)
	if err != nil {
		return fmt.Errorf("error connecting to DB: %w", err)
	}
	defer conn.Close(ctx)

	columns := db.MakeColumnString(columnNames)
	values := db.MakeValuesString(columnNames)

	if _, err := conn.Exec(ctx,
		fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", table, columns, values),
		t,
		room,
		data.Temperature,
		data.Humidity,
		data.Battery,
	); err != nil {
		return fmt.Errorf("error writing row to DB: %w", err)
	}

	return nil
}
