package clockdrift

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	log := logrus.New()
	config := &ClockDriftConfig{
		NTPServer:    "pool.ntp.org",
		SyncInterval: 5 * time.Minute,
	}

	service := NewService(log, config)
	assert.NotNil(t, service)
	assert.Equal(t, config, service.config)
	assert.NotNil(t, service.scheduler)
}

func TestService_StartStop(t *testing.T) {
	log := logrus.New()
	config := &ClockDriftConfig{
		NTPServer:    "pool.ntp.org",
		SyncInterval: 5 * time.Minute,
	}

	service := NewService(log, config)
	ctx := context.Background()

	// Test Start.
	err := service.Start(ctx)
	require.NoError(t, err)
	assert.Greater(t, len(service.scheduler.Jobs()), 0)

	// Test Stop.
	err = service.Stop(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, len(service.scheduler.Jobs()))
}

func TestService_GetDrift(t *testing.T) {
	log := logrus.New()
	config := &ClockDriftConfig{
		NTPServer:    "pool.ntp.org",
		SyncInterval: 5 * time.Minute,
	}

	service := NewService(log, config)

	// Initial drift should be zero
	assert.Equal(t, time.Duration(0), service.GetDrift())

	// Set a mock drift
	service.mu.Lock()
	service.clockDrift = 1 * time.Second
	service.mu.Unlock()

	// Check if we can read the drift
	assert.Equal(t, 1*time.Second, service.GetDrift())
}

func TestService_Now(t *testing.T) {
	log := logrus.New()
	config := &ClockDriftConfig{
		NTPServer:    "pool.ntp.org",
		SyncInterval: 5 * time.Minute,
	}

	service := NewService(log, config)

	// Set a known drift.
	service.mu.Lock()
	service.clockDrift = 1 * time.Second
	service.mu.Unlock()

	// Get time before and after calling Now().
	beforeTime := time.Now()
	driftTime := service.Now()
	afterTime := time.Now()

	// The drift-adjusted time should be greater than beforeTime + drift
	// and less than afterTime + drift.
	assert.True(t, driftTime.After(beforeTime.Add(service.GetDrift())))
	assert.True(t, driftTime.Before(afterTime.Add(service.GetDrift())))
}

func TestService_SyncDrift(t *testing.T) {
	log := logrus.New()
	config := &ClockDriftConfig{
		NTPServer:    "pool.ntp.org",
		SyncInterval: 5 * time.Minute,
	}

	service := NewService(log, config)

	// Test actual NTP sync.
	err := service.syncDrift()
	require.NoError(t, err)

	// Verify that we got some drift value.
	drift := service.GetDrift()
	assert.NotEqual(t, time.Duration(0), drift)
}

func TestService_SyncDriftWithLargeDrift(t *testing.T) {
	log := logrus.New()
	config := &ClockDriftConfig{
		NTPServer:    "pool.ntp.org",
		SyncInterval: 5 * time.Minute,
	}

	service := NewService(log, config)

	// Set a large drift manually to test warning.
	service.mu.Lock()
	service.clockDrift = 3 * time.Second
	service.mu.Unlock()

	// Verify the drift is large.
	drift := service.GetDrift()
	assert.Equal(t, 3*time.Second, drift)
}

func TestService_Interface(t *testing.T) {
	var _ ClockDrift = (*Service)(nil)
}
