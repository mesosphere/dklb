package admission

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// bindAddress is the address ("host:port") which to bind to.
	bindAddress = "0.0.0.0:8443"
	// healthzPath is the path where the "health" endpoint is served.
	healthzPath = "/healthz"
)

// Webhook represents an admission webhook.
type Webhook struct {
	// tlsCertificate is the TLS certificate to use for the server.
	tlsCertificate tls.Certificate
}

// NewWebhook creates a new instance of the admission webhook.
func NewWebhook(tlsCertificate tls.Certificate) *Webhook {
	return &Webhook{
		tlsCertificate: tlsCertificate,
	}
}

// Run starts the HTTP server that backs the admission webhook.
func (w *Webhook) Run(stopCh chan struct{}) error {
	// Create an HTTP server and register handler functions to back the admission webhook.
	mux := http.NewServeMux()
	mux.HandleFunc(healthzPath, handleHealthz)
	srv := http.Server{
		Addr:    bindAddress,
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{w.tlsCertificate},
		},
	}

	// Shutdown the server when stopCh is closed.
	go func() {
		<-stopCh
		ctx, fn := context.WithTimeout(context.Background(), 5*time.Second)
		defer fn()
		if err := srv.Shutdown(ctx); err != nil {
			log.Errorf("failed to shutdown the admission webhook: %v", err)
		} else {
			log.Debug("the admission webhook has been shutdown")
		}
	}()

	// Start listening and serving requests.
	log.Debug("starting the admission webhook")
	if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
