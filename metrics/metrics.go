package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace             = "backmon"
	subsystem             = "backup"
	subsystemEnvironments = "environments"
	subsystemDisks        = "disks"
)

var (
	registry = prometheus.NewRegistry()
	handler  = promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.HTTPErrorOnError,
	})
)

func Handler() http.Handler {
	return handler
}
