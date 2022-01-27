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
	Temperature float32
	Humidity    float32
	Battery     uint16
	BatteryVolt float32
}

func ParseMessage(ads *hci.AdStructure) (*Data, error) {
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
