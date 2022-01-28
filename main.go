package main

import (
	"flag"
	"log"

	hcLog "github.com/brutella/hc/log"
	"github.com/piger/sensor-probe/internal/config"
	"github.com/piger/sensor-probe/internal/probe"
)

var (
	device     = flag.String("device", "hci0", "Bluetooth device")
	configFile = flag.String("config", "sensor-probe.toml", "Path to the configuration file")
	debugHk    = flag.Bool("debug-hk", false, "Enable HomeKit debugging")
)

func main() {
	flag.Parse()

	cfg, err := config.ReadConfig(*configFile)
	if err != nil {
		log.Fatalf("error: %s", err)
	}

	probe := probe.New(*device, cfg)

	if *debugHk {
		hcLog.Debug.Enable()
	}

	if err := probe.Run(); err != nil {
		log.Fatal(err)
	}
}
