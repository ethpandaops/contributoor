package application

import (
	"context"

	"github.com/ethpandaops/contributoor/internal/clockdrift"
)

// initClockDrift initializes the clock drift service if not already provided.
// This service monitors time synchronization with NTP servers.
func (a *Application) initClockDrift(ctx context.Context) error {
	if a.clockDrift != nil {
		// Already initialized
		return nil
	}

	clockDriftService := clockdrift.NewService(a.log, &clockdrift.ClockDriftConfig{
		NTPServer:    a.ntpServer,
		SyncInterval: a.clockDriftSyncInterval,
	})

	if err := clockDriftService.Start(ctx); err != nil {
		return err
	}

	a.clockDrift = clockDriftService
	a.log.Info("Clock drift service initialized")

	return nil
}

// ClockDrift returns the clock drift service instance.
// This is useful for testing or external monitoring.
func (a *Application) ClockDrift() clockdrift.ClockDrift {
	return a.clockDrift
}
