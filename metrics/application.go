package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type ApplicationMetrics struct {
	EnvironmentsTotal prometheus.Gauge
	// only make the total disks updatable from the disk metrics method
	disksTotal prometheus.Gauge
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
		}

		registry.MustRegister(applicationMetrics.EnvironmentsTotal)
		registry.MustRegister(applicationMetrics.disksTotal)
	})

	return applicationMetrics
}
