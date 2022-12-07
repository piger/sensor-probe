package config

import (
	"fmt"
	"os"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/pelletier/go-toml/v2"
)

// Config is the main configuration of this program.
type Config struct {
	HomeKit  HomeKit        `toml:"homekit"`
	Sensors  []SensorConfig `toml:"sensors"`
	Interval duration       `toml:"interval"`
	DBConfig string         `toml:"dbconfig"`
}

func (c Config) Validate() error {
	err := validation.ValidateStruct(&c,
		validation.Field(&c.HomeKit, validation.Required),
		validation.Field(&c.Sensors, validation.Required),
		validation.Field(&c.Interval, validation.Required),
		validation.Field(&c.DBConfig, validation.Required),
	)
	return err
}

type HomeKit struct {
	Pin     string `toml:"pin"`
	Port    int    `toml:"port"`
	SetupID string `toml:"setup_id"`
}

func (hk HomeKit) Validate() error {
	err := validation.ValidateStruct(&hk,
		validation.Field(&hk.Pin, validation.Required),
		validation.Field(&hk.Port, validation.Required),
		validation.Field(&hk.SetupID, validation.Required),
	)
	return err
}

// SensorConfig contains the configuration of a single sensor.
type SensorConfig struct {
	Name     string `toml:"name"`
	MAC      string `toml:"mac"`
	Firmware string `toml:"firmware"`
	DBTable  string `toml:"dbtable"`
}

func (sc SensorConfig) Validate() error {
	err := validation.ValidateStruct(&sc,
		validation.Field(&sc.Name, validation.Required),
		validation.Field(&sc.MAC, validation.Required, is.MAC),
		validation.Field(&sc.Firmware, validation.Required, validation.In("custom", "ruuviv5")),
		validation.Field(&sc.DBTable, validation.Required),
	)
	return err
}

type duration struct {
	time.Duration
}

func (i *duration) UnmarshalText(text []byte) (err error) {
	i.Duration, err = time.ParseDuration(string(text))
	return err
}

func ReadConfig(filename string) (*Config, error) {
	fh, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening configuration file %q: %w", filename, err)
	}
	defer fh.Close()

	d := toml.NewDecoder(fh)
	d.DisallowUnknownFields()

	var config Config
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}
