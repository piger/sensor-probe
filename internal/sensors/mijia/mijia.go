// Xiaomi Thermometer LYWSD03MMC
// https://github.com/atc1441/ATC_MiThermometer

package mijia

import (
	"bytes"
	"encoding/binary"

	"gitlab.com/jtaimisto/bluewalker/hci"
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
	Temperature float64
	Humidity    int
	Battery     int
	BatteryVolt int
}

func ParseMessage(ads *hci.AdStructure) (*Data, error) {
	var p payload
	buf := bytes.NewBuffer(ads.Data)
	if err := binary.Read(buf, binary.BigEndian, &p); err != nil {
		return nil, err
	}

	data := Data{
		Temperature: float64(p.Temperature) / 10.0,
		Humidity:    int(p.Humidity),
		Battery:     int(p.Battery),
		BatteryVolt: int(p.BatterymVolt),
	}
	return &data, nil
}
