package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Config is the main configuration of this program.
type Config struct {
	HomeKit  HomeKit        `toml:"homekit"`
	Sensors  []SensorConfig `toml:"sensors"`
	Interval duration       `toml:"interval"`
	DBConfig string         `toml:"dbconfig"`
}

type HomeKit struct {
	Pin     string `toml:"pin"`
	Port    int    `toml:"port"`
	SetupID string `toml:"setup_id"`
}

// SensorConfig contains the configuration of a single sensor.
type SensorConfig struct {
	Name     string `toml:"name"`
	MAC      string `toml:"mac"`
	Firmware string `toml:"firmware"`
	DBTable  string `toml:"dbtable"`
}

type duration struct {
	time.Duration
}

func (i *duration) UnmarshalText(text []byte) error {
	var err error
	i.Duration, err = time.ParseDuration(string(text))
	return err
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

	if config.HomeKit.Pin == "" {
		return nil, errors.New("missing pin in homekit")
	}
	if config.HomeKit.Port == 0 {
		return nil, errors.New("missing port in homekit")
	}
	if config.HomeKit.SetupID == "" {
		return nil, errors.New("missing SetupID in homekit")
	}

	for i, sensor := range config.Sensors {
		if sensor.Name == "" {
			return nil, fmt.Errorf("missing name for sensor %d", i)
		}
		if sensor.MAC == "" {
			return nil, fmt.Errorf("missing MAC for sensor %d", i)
		}
		if sensor.DBTable == "" {
			return nil, fmt.Errorf("missing DBTable for sensor %d", i)
		}
		config.Sensors[i].MAC = strings.ToUpper(sensor.MAC)
	}

	return &config, nil
}
