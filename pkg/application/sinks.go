package application

import (
	"context"
	"fmt"

	"github.com/ethpandaops/contributoor/internal/sinks"
	"github.com/sirupsen/logrus"
)

// initSinks creates the event sinks based on configuration.
// In debug mode, it uses stdout sink. Otherwise, it creates the configured output sink.
// Note: Sinks are started by the BeaconWrapper when the beacon node starts.
func (a *Application) initSinks(ctx context.Context, log logrus.FieldLogger, traceID string) ([]sinks.ContributoorSink, error) {
	eventSinks := make([]sinks.ContributoorSink, 0)

	if a.debug {
		// Debug mode - use stdout sink
		stdoutSink, err := sinks.NewStdoutSink(log, a.config, traceID)
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout sink: %w", err)
		}

		eventSinks = append(eventSinks, stdoutSink)

		log.Info("Using stdout sink (debug mode)")
	} else {
		// Production mode - use xatu sink
		xatuSink, err := sinks.NewXatuSink(log, a.config, traceID)
		if err != nil {
			return nil, fmt.Errorf("failed to create xatu sink: %w", err)
		}

		eventSinks = append(eventSinks, xatuSink)

		log.Info("Using xatu sink")
	}

	return eventSinks, nil
}
