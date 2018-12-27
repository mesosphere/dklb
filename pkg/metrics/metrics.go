package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/mesosphere/dklb/pkg/constants"
)

const (
	// controllerNameLabel is the name of the label used to hold the name of a controller.
	controllerNameLabel = "controller_name"
	// lastSyncTimestampKey is the name of the metric used to hold the timestamp at which a controller last synced a resource.
	lastSyncTimestampKey = "last_sync_timestamp"
	// resourceKeyLabel is the name of the label used to hold the key of a resource.
	resourceKeyLabel = "resource_key"
	// syncDurationSecondsKey is the name of the metric used to hold the time taken to sync resources,
	syncDurationSecondsKey = "sync_duration_seconds"
	// totalSyncsKey is the name of the metric used to hold the total number of times a controller synced a resource.
	totalSyncsKey = "total_syncs"
)

var (
	// lastSyncTimestamp holds the timestamp at which a controller last synced a resource.
	lastSyncTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: constants.ComponentName,
		Name:      lastSyncTimestampKey,
		Help:      "The timestamp at which a controller last synced a resource",
	}, []string{controllerNameLabel, resourceKeyLabel})
	// syncDuration holds the time taken to sync resources.
	syncDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: constants.ComponentName,
		Name:      syncDurationSecondsKey,
		Help:      "The time taken to sync resources",
	}, []string{controllerNameLabel})
	// totalSyncs holds the total number of times a controller synced a resource.
	totalSyncs = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: constants.ComponentName,
		Name:      totalSyncsKey,
		Help:      "The total number of times a controller synced a resource",
	}, []string{controllerNameLabel, resourceKeyLabel})
)

var (
	// register is used to guarantee that metrics registering happens only once.
	register sync.Once
)

// RegisterMetrics initializes the metrics registry by registering metrics.
func RegisterMetrics() {
	register.Do(func() {
		prometheus.MustRegister(lastSyncTimestamp)
		prometheus.MustRegister(syncDuration)
		prometheus.MustRegister(totalSyncs)
	})
}

// RecordSyncDuration records the time taken by a single iteration of the specified controller.
func RecordSyncDuration(controllerName string, startTime time.Time) {
	syncDuration.WithLabelValues(
		controllerName).Observe(float64(time.Since(startTime)) / float64(time.Second))
}

// RecordSync records an attempt made by the specified controller at synchronizing (reconciling) the resource with the provided key.
func RecordSync(controllerName, resourceKey string) {
	lastSyncTimestamp.WithLabelValues(
		controllerName,
		resourceKey).Set(float64(time.Now().UTC().UnixNano()))
	totalSyncs.WithLabelValues(
		controllerName,
		resourceKey).Inc()
}
