package signals

import (
	"os"
	"os/signal"
	"syscall"
)

// SetupSignalHandler registers a listener for the SIGTERM and SIGINT signals.
// A channel is returned, which is closed on one of these signals.
// If a second signal is caught, the program is terminated immediately with exit code 1.
func SetupSignalHandler() chan struct{} {
	stopCh := make(chan struct{})
	termCh := make(chan os.Signal, 2)
	// Notify termCh of SIGINT and SIGTERM.
	signal.Notify(termCh, syscall.SIGINT, syscall.SIGTERM)
	// Wait for a signal to be received.
	go func() {
		<-termCh
		// The first signal was received, so we close the channel.
		close(stopCh)
		<-termCh
		// The second signal was received, so we exit immediately.
		os.Exit(1)
	}()
	return stopCh
}
