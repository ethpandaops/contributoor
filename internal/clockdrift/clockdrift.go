package clockdrift

import (
	"context"
	"sync"
	"time"

	"github.com/beevik/ntp"
	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
)

// ClockDrift is the interface that wraps the methods for the clock drift service.
type ClockDrift interface {
	// GetDrift returns the current clock drift.
	GetDrift() time.Duration
	// Now returns the current time adjusted for clock drift.
	Now() time.Time
}

// Service is the clock drift service.
type Service struct {
	config     *ClockDriftConfig
	log        logrus.FieldLogger
	scheduler  *gocron.Scheduler
	clockDrift time.Duration
	mu         sync.RWMutex
}

// ClockDriftConfig is the configuration for the clock drift service.
type ClockDriftConfig struct {
	// NTP server to use for syncing
	NTPServer string `yaml:"ntpServer" default:"pool.ntp.org"`
	// How often to sync clock drift
	SyncInterval time.Duration `yaml:"syncInterval" default:"5m"`
}

// NewService creates a new clock drift service.
func NewService(log logrus.FieldLogger, config *ClockDriftConfig) *Service {
	return &Service{
		config:    config,
		log:       log.WithField("service", "clockdrift"),
		scheduler: gocron.NewScheduler(time.Local),
	}
}

// Start starts the clock drift service.
func (s *Service) Start(_ context.Context) error {
	if err := s.syncDrift(); err != nil {
		s.log.WithError(err).Error("Failed initial clock drift sync")
	}

	if _, err := s.scheduler.Every(s.config.SyncInterval).Do(func() {
		if err := s.syncDrift(); err != nil {
			s.log.WithError(err).Error("Failed to sync clock drift")
		}
	}); err != nil {
		return err
	}

	s.scheduler.StartAsync()

	return nil
}

// Stop stops the clock drift service.
func (s *Service) Stop(_ context.Context) error {
	s.scheduler.Stop()

	return nil
}

// GetDrift returns the current clock drift.
func (s *Service) GetDrift() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.clockDrift
}

// Now returns the current time adjusted for clock drift.
func (s *Service) Now() time.Time {
	return time.Now().Add(s.GetDrift())
}

func (s *Service) syncDrift() error {
	response, err := ntp.Query(s.config.NTPServer)
	if err != nil {
		return err
	}

	if err = response.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	s.clockDrift = response.ClockOffset
	s.mu.Unlock()

	s.log.WithField("drift", s.clockDrift).Info("Updated clock drift")

	if s.clockDrift > 2*time.Second || s.clockDrift < -2*time.Second {
		s.log.WithField("drift", s.clockDrift).Warn("Large clock drift detected")
	}

	return nil
}
