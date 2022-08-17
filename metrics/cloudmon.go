package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type CloudmonMetrics struct {
	EnvironmentsTotal prometheus.Gauge
	// only make the total disks updatable from the disk metrics method
	disksTotal prometheus.Gauge
}

var (
	cloudmonMetrics *CloudmonMetrics
	once            sync.Once
)

func GetCloudmonMetrics() *CloudmonMetrics {
	once.Do(func() {
		cloudmonMetrics = &CloudmonMetrics{
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

		registry.MustRegister(cloudmonMetrics.EnvironmentsTotal)
		registry.MustRegister(cloudmonMetrics.disksTotal)
	})

	return cloudmonMetrics
}
