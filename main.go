package main

import (
	"flag"
	"log"

	"github.com/piger/sensor-probe/internal/config"
	"github.com/piger/sensor-probe/internal/probe"
)

var (
	device     = flag.String("device", "hci0", "Bluetooth device")
	configFile = flag.String("config", "sensor-probe.toml", "Path to the configuration file")
)

func main() {
	flag.Parse()

	cfg, err := config.ReadConfig(*configFile)
	if err != nil {
		log.Fatalf("error: %s", err)
	}

	probe := probe.New(*device, cfg)
	if err := probe.Initialize(); err != nil {
		log.Fatalf("initializing probe: %s", err)
	}

	if err := probe.Run(); err != nil {
		log.Fatal(err)
	}
}
