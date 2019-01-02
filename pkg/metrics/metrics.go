package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"github.com/mesosphere/dklb/pkg/constants"
)

const (
	// bindAddr is the address (host and port) at which to expose the "/metrics" handler.
	bindAddr = "0.0.0.0:10250"
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

func init() {
	// Register metrics.
	prometheus.MustRegister(lastSyncTimestamp)
	prometheus.MustRegister(syncDuration)
	prometheus.MustRegister(totalSyncs)
	// Start the HTTP server that will expose application-level metrics.
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(bindAddr, nil); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
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
