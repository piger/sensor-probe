package probe

import "github.com/prometheus/client_golang/prometheus"

var (
	temperatureMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sensor_probe_temperature",
			Help: "The current temperature measured by the Xiaomi sensor",
		},
		[]string{"name"},
	)

	humidityMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sensor_probe_humidity",
			Help: "The current humidity measured by the Xiaomi sensor",
		},
		[]string{"name"},
	)

	batteryMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sensor_probe_battery",
			Help: "The battery level of the Xiaomi sensor",
		},
		[]string{"name"},
	)
)

func init() {
	prometheus.MustRegister(temperatureMetric)
	prometheus.MustRegister(humidityMetric)
	prometheus.MustRegister(batteryMetric)
}
