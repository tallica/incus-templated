package templated

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	renderFailedGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: name,
		Name:      "render_failed",
		Help:      "1 while a target fails to render and its live file is held as it was.",
	}, []string{"target"})

	renderLeftOutGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: name,
		Name:      "render_instances_left_out",
		Help:      "Number of instances left out of a target's last render because they failed to render.",
	}, []string{"target"})
)
