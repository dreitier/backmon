package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ApplicationMetrics struct {
	EnvironmentsTotal prometheus.Gauge
	// only make the total disks updatable from the disk metrics method
	disksTotal prometheus.Gauge
	// same semantics and naming as the configuration reload metrics of Prometheus and Alertmanager
	configLastReloadSuccessful       prometheus.Gauge
	configLastReloadSuccessTimestamp prometheus.Gauge
}

// ConfigReloaded records the outcome of loading the configuration, at startup as well as on each reload
func (m *ApplicationMetrics) ConfigReloaded(successful bool) {
	if !successful {
		m.configLastReloadSuccessful.Set(0)
		return
	}

	m.configLastReloadSuccessful.Set(1)
	m.configLastReloadSuccessTimestamp.Set(float64(time.Now().Unix()))
}

var (
	applicationMetrics *ApplicationMetrics
	once               sync.Once
)

func GetApplicationMetrics() *ApplicationMetrics {
	once.Do(func() {
		applicationMetrics = &ApplicationMetrics{
			EnvironmentsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystemEnvironments,
				Name:      "total",
				Help:      "Total number of environments",
			}),
			disksTotal: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystemDisks,
				Name:      "total",
				Help:      "Total number of registered disks",
			}),
			configLastReloadSuccessful: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystemConfig,
				Name:      "last_reload_successful",
				Help:      "Whether the last attempt to (re)load the configuration file was successful (1) or not (0); after a failed reload the previous configuration stays active",
			}),
			configLastReloadSuccessTimestamp: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystemConfig,
				Name:      "last_reload_success_timestamp_seconds",
				Help:      "Unix timestamp of the last successful (re)load of the configuration file",
			}),
		}

		registry.MustRegister(applicationMetrics.EnvironmentsTotal)
		registry.MustRegister(applicationMetrics.disksTotal)
		registry.MustRegister(applicationMetrics.configLastReloadSuccessful)
		registry.MustRegister(applicationMetrics.configLastReloadSuccessTimestamp)
	})

	return applicationMetrics
}
