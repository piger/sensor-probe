package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	hcLog "github.com/brutella/hc/log"
	"github.com/pelletier/go-toml/v2"
	"github.com/piger/sensor-probe/internal/config"
	"github.com/piger/sensor-probe/internal/probe"
)

var (
	device      = flag.String("device", "hci0", "Bluetooth device")
	configFile  = flag.String("config", "sensor-probe.toml", "Path to the configuration file")
	debugHk     = flag.Bool("debug-hk", false, "Enable HomeKit debugging")
	showVersion = flag.Bool("version", false, "Print version information")
)

func run() error {
	cfg, err := config.ReadConfig(*configFile)
	if err != nil {
		return err
	}

	probe := probe.New(*device, cfg)

	if *debugHk {
		hcLog.Debug.Enable()
	}

	return probe.Run()
}

func main() {
	flag.Parse()

	if *showVersion {
		version, err := getVersion()
		if err != nil {
			panic(err)
		}
		fmt.Println(version)
		return
	}

	if err := run(); err != nil {
		var derr *toml.DecodeError
		var sterr *toml.StrictMissingError
		switch {
		case errors.As(err, &derr):
			fmt.Println("configuration error:")
			fmt.Println(derr.String())
			row, col := derr.Position()
			fmt.Printf("error at row %d, column %d\n", row, col)
			os.Exit(1)
		case errors.As(err, &sterr):
			fmt.Println("configuration error:")
			fmt.Println(sterr.String())
			os.Exit(1)
		default:
			log.Fatalf("error: %s", err)
		}
	}
}
