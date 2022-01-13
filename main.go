package main

import (
	"flag"
	"log"
	"net/http"

	hcLog "github.com/brutella/hc/log"
	"github.com/piger/sensor-probe/internal/config"
	"github.com/piger/sensor-probe/internal/probe"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	device      = flag.String("device", "hci0", "Bluetooth device")
	configFile  = flag.String("config", "sensor-probe.toml", "Path to the configuration file")
	metricsAddr = flag.String("metrics-addr", "localhost:30333", "Listen address to expose metrics")
	debugHk     = flag.Bool("debug-hk", false, "Enable HomeKit debugging")
)

func main() {
	flag.Parse()

	cfg, err := config.ReadConfig(*configFile)
	if err != nil {
		log.Fatalf("error: %s", err)
	}

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(*metricsAddr, nil); err != nil {
			log.Printf("http listener error: %s", err)
		}
	}()

	probe := probe.New(*device, cfg)
	if err := probe.Initialize(); err != nil {
		log.Fatalf("initializing probe: %s -- remember to run: sudo hciconfig hci0 down", err)
	}

	if *debugHk {
		hcLog.Debug.Enable()
	}

	if err := probe.Run(); err != nil {
		log.Fatal(err)
	}
}
