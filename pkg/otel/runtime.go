package otel

import (
	"time"

	hostmetrics "go.opentelemetry.io/contrib/instrumentation/host"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
)

// StartRuntimeMetrics initializes OpenTelemetry runtime and host metrics collection
// This collects important system metrics like:
// - Memory allocation/usage
// - GC statistics
// - CPU utilization
// - Network utilization
// - Disk I/O
func StartRuntimeMetrics() error {
	// Start runtime metrics collection (memory, GC, etc)
	if err := runtime.Start(
		runtime.WithMinimumReadMemStatsInterval(time.Second*30),
	); err != nil {
		return err
	}

	// Start host metrics collection (CPU, memory, network, disk)
	if err := hostmetrics.Start(); err != nil {
		return err
	}

	return nil
}
