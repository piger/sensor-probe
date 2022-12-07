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

var (
	device     = flag.String("device", "hci0", "Bluetooth device")
	configFile = flag.String("config", "sensor-probe.toml", "Path to the configuration file")
	debugHk    = flag.Bool("debug-hk", false, "Enable HomeKit debugging")
)

func run() error {
	flag.Parse()

	cfg, err := config.ReadConfig(*configFile)
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
		}
		return err
	}

	probe := probe.New(*device, cfg)

	if *debugHk {
		hcLog.Debug.Enable()
	}

	return probe.Run()
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("error: %s", err)
	}
}
