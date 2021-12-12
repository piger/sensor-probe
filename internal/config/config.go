package config

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Config is the main configuration of this program.
type Config struct {
	Sensors []SensorConfig `toml:"sensors"`
}

// SensorConfig contains the configuration of a single sensor.
type SensorConfig struct {
	Name     string
	MAC      string
	Firmware string
}

func ReadConfig(filename string) (*Config, error) {
	fh, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening configuration file %q: %w", filename, err)
	}
	defer fh.Close()

	data, err := io.ReadAll(fh)
	if err != nil {
		return nil, fmt.Errorf("reading configuration file %q: %w", filename, err)
	}

	var config Config
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing configuration file %q: %w", filename, err)
	}

	for _, sensor := range config.Sensors {
		sensor.MAC = strings.ToUpper(sensor.MAC)
	}

	return &config, nil
}
