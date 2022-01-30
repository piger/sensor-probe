// Ruuvi tag
// https://github.com/ruuvi/ruuvi-sensor-protocols/blob/master/dataformat_05.md

package ruuvi

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

// v5 format
type payload struct {
	UUID            uint16 // 0x0499, manufacturer ID
	Format          int8
	Temperature     int16
	Humidity        uint16
	Pressure        uint16
	AccelerationX   int16
	AccelerationY   int16
	AccelerationZ   int16
	PowerInfo       uint16 // it's composed
	MovementCounter uint8
	Sequence        uint16
	MAC             [6]uint8
}

type Data struct {
	Temperature   float32
	Humidity      float32
	Pressure      int
	AccelerationX float32
	AccelerationY float32
	AccelerationZ float32
	Voltage       int
	TxPower       int
	MoveCount     int
	Seq           int
}

var columnNames = []string{
	"time",
	"temperature",
	"humidity",
	"pressure",
	"voltage",
	"txpower",
}

// TODO is the data prefixed by the manufacturer ID?? 0x0499 (2 bytes)
// and if so should we skip it? how does the mijia parser handle it??

func parseMessage(ads *hci.AdStructure) (*Data, error) {
	var p payload
	buf := bytes.NewBuffer(ads.Data)
	if err := binary.Read(buf, binary.BigEndian, &p); err != nil {
		return nil, err
	}

	if p.Format != 5 {
		return nil, fmt.Errorf("wrong data format: %d", p.Format)
	}

	// 0xFFFF - 0x001F (mask for 5 bits) = 0xFFE0
	tmp := (p.PowerInfo & 0xFFE0) >> 5
	voltage := int(1600 + tmp)

	tmp = (p.PowerInfo & 0x001F)
	txpower := -40 + int((tmp * 2))

	// v5 format
	// XXX need to compare Voltage and TxPower with bluewalker, to see if I'm parsing correctly
	data := Data{
		Temperature:   float32(p.Temperature) * 0.005,
		Humidity:      float32(p.Humidity) * 0.0025,
		Pressure:      int(p.Pressure) + 50000,
		AccelerationX: float32(p.AccelerationX) / 1000,
		AccelerationY: float32(p.AccelerationY) / 1000,
		AccelerationZ: float32(p.AccelerationZ) / 1000,
		Voltage:       voltage,
		TxPower:       txpower,
		MoveCount:     int(p.MovementCounter),
		Seq:           int(p.Sequence),
	}

	return &data, nil
}

const UUID = 0x0499

func checkReport(r *hci.AdStructure) bool {
	return r.Typ == hci.AdManufacturerSpecific && len(r.Data) >= 2 && binary.LittleEndian.Uint16(r.Data) == UUID
}

type RuuviSensor struct {
	*sensors.Sensor
}

func NewRuuviSensor(config *config.SensorConfig, id uint64) *RuuviSensor {
	info := accessory.Info{
		Name:         config.Name,
		Model:        "",
		SerialNumber: "",
		Manufacturer: "",
		ID:           id,
	}

	acc := homekit.NewTemperatureHumiditySensor(info)
	s := sensors.NewSensor(config, acc)

	return &RuuviSensor{s}
}

func (rv *RuuviSensor) Update(ctx context.Context, report *host.ScanReport, dbconfig string) error {
	for _, ads := range report.Data {
		if checkReport(ads) {
			if err := rv.handleBroadcast(ctx, ads, dbconfig); err != nil {
				log.Print(err)
			}
		}
	}
	return nil
}

func (rv *RuuviSensor) handleBroadcast(ctx context.Context, b *hci.AdStructure, dbconfig string) error {
	data, err := parseMessage(b)
	if err != nil {
		return err
	}

	now := time.Now()
	log.Printf("%q (%s): T=%.2f H=%.2f%% P=%d B=%d%%", rv.Name, rv.MAC, data.Temperature, data.Humidity, data.Pressure, data.TxPower)

	if rv.LastUpdateHomeKit.IsZero() || now.Sub(rv.LastUpdateHomeKit) >= sensors.UpdateDelayHk {
		rv.SetTemperature(float64(data.Temperature))
		rv.SetHumidity(float64(data.Humidity))
		rv.LastUpdateHomeKit = now
	}

	if rv.LastUpdateDB.IsZero() || now.Sub(rv.LastUpdateDB) >= sensors.UpdateDelayDB {
		if err := writeDBRow(ctx, now, data, dbconfig, rv.DBTable); err != nil {
			return err
		}
		rv.LastUpdateDB = now
	}

	return nil
}

func writeDBRow(ctx context.Context, t time.Time, data *Data, dburl, table string) error {
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
		data.Temperature,
		data.Humidity,
		data.Pressure,
		data.Voltage,
		data.TxPower,
	); err != nil {
		return fmt.Errorf("error writing row to DB: %w", err)
	}

	return nil
}
