package clockdrift

import (
	"context"
	"sync"
	"time"

	"github.com/beevik/ntp"
	"github.com/go-co-op/gocron/v2"
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
	scheduler  gocron.Scheduler
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
	s, _ := gocron.NewScheduler(gocron.WithLocation(time.Local))

	return &Service{
		config:    config,
		log:       log.WithField("service", "clockdrift"),
		scheduler: s,
	}
}

// Start starts the clock drift service.
func (s *Service) Start(_ context.Context) error {
	if err := s.syncDrift(); err != nil {
		s.log.WithError(err).Error("Failed initial clock drift sync")
	}

	if _, err := s.scheduler.NewJob(
		gocron.DurationJob(s.config.SyncInterval),
		gocron.NewTask(
			func() {
				if err := s.syncDrift(); err != nil {
					s.log.WithError(err).Error("Failed to sync clock drift")
				}
			},
		),
	); err != nil {
		return err
	}

	s.scheduler.Start()

	return nil
}

// Stop stops the clock drift service.
func (s *Service) Stop(_ context.Context) error {
	return s.scheduler.Shutdown()
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

	if rspErr := response.Validate(); rspErr != nil {
		return rspErr
	}

	s.mu.Lock()
	s.clockDrift = response.ClockOffset
	s.mu.Unlock()

	s.log.WithField("drift", s.clockDrift).Debug("Updated clock drift")

	if s.clockDrift > 2*time.Second || s.clockDrift < -2*time.Second {
		s.log.WithField("drift", s.clockDrift).Warn("Large clock drift detected")
	}

	return nil
}
