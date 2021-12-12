package probe

import "github.com/prometheus/client_golang/prometheus"

var (
	temperatureMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sensor_probe_temperature",
			Help: "The current temperature measured by the Xiaomi sensor",
		},
		[]string{"name", "address"},
	)

	humidityMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sensor_probe_humidity",
			Help: "The current humidity measured by the Xiaomi sensor",
		},
		[]string{"name", "address"},
	)

	batteryMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sensor_probe_battery",
			Help: "The battery level of the Xiaomi sensor",
		},
		[]string{"name", "address"},
	)
)

func init() {
	prometheus.MustRegister(temperatureMetric)
	prometheus.MustRegister(humidityMetric)
	prometheus.MustRegister(batteryMetric)
}
