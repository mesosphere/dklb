package backends

import (
	"context"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// bindAddress is the address ("host:port") which to bind to.
	bindAddress = "0.0.0.0:8080"
)

// DefaultBackend represents the default backend.
type DefaultBackend struct {
}

// NewDefaultBackend creates a new instance of the default backend.
func NewDefaultBackend() *DefaultBackend {
	return &DefaultBackend{}
}

// Run starts the HTTP server that backs the default backend.
func (db *DefaultBackend) Run(stopCh chan struct{}) error {
	// Configure the HTTP server.
	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)
	srv := http.Server{
		Addr:    bindAddress,
		Handler: mux,
	}

	// Shutdown the server when stopCh is closed.
	go func() {
		<-stopCh
		ctx, fn := context.WithTimeout(context.Background(), 5*time.Second)
		defer fn()
		if err := srv.Shutdown(ctx); err != nil {
			log.Errorf("failed to shutdown the default backend: %v", err)
		} else {
			log.Debug("the default backend has been shutdown")
		}
	}()

	// Start listening and serving requests.
	log.Debug("starting the default backend")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// handle handles the specified request by responding with "503 SERVICE UNAVAILABLE" and an error message.
func handle(res http.ResponseWriter, _ *http.Request) {
	http.Error(res, "No backend is available to service this request.", http.StatusServiceUnavailable)
}
