// Ruuvi tag
// https://github.com/ruuvi/ruuvi-sensor-protocols/blob/master/dataformat_05.md

package ruuvi

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"gitlab.com/jtaimisto/bluewalker/hci"
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

// TODO is the data prefixed by the manufacturer ID?? 0x0499 (2 bytes)
// and if so should we skip it? how does the mijia parser handle it??

func ParseMessage(ads *hci.AdStructure) (*Data, error) {
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
