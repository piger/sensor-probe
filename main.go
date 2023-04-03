package main

import (
	"errors"
	"flag"
	"fmt"
	"log"

	hcLog "github.com/brutella/hc/log"
	"github.com/pelletier/go-toml/v2"
	"github.com/piger/sensor-probe/internal/config"
	"github.com/piger/sensor-probe/internal/probe"
)

// readConfig wraps the clunky error handling of go-toml to show nice error messages.
func readConfig(filename string) (*config.Config, error) {
	cfg, err := config.ReadConfig(filename)
	if err != nil {
		var derr *toml.DecodeError
		var sterr *toml.StrictMissingError
		switch {
		case errors.As(err, &derr):
			fmt.Println("configuration error:")
			fmt.Println(derr.String())
			row, col := derr.Position()
			fmt.Printf("error at row %d, column %d\n", row, col)
		case errors.As(err, &sterr):
			fmt.Println("configuration error:")
			fmt.Println(sterr.String())
		default:
			fmt.Printf("error: %s\n", err)
		}

		return nil, err
	}

	return cfg, nil
}

func main() {
	var (
		deviceFlag       string
		configFileFlag   string
		debugHomeKitFlag bool
		versionFlag      bool
	)
	flag.StringVar(&deviceFlag, "device", "hci0", "Name of the Bluetooth device used to listen for BLE messages")
	flag.StringVar(&configFileFlag, "config", "sensor-probe.toml", "Configuration file name")
	flag.BoolVar(&debugHomeKitFlag, "debug-hk", false, "Enable to turn on the debugging for the HomeKit subsystem")
	flag.BoolVar(&versionFlag, "version", false, "Show the program's version")
	flag.Parse()

	if versionFlag {
		version, err := getVersion()
		if err != nil {
			panic(err)
		}
		fmt.Println(version)
		return
	}

	cfg, err := readConfig(configFileFlag)
	if err != nil {
		log.Fatalf("error reading configuration: %s", err)
	}

	if debugHomeKitFlag {
		hcLog.Debug.Enable()
	}

	p := probe.New(deviceFlag, cfg)
	if err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
