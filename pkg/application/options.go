package application

import (
	"time"

	"github.com/ethpandaops/contributoor/internal/clockdrift"
	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/sirupsen/logrus"
)

// Options contains all configuration options for creating an Application.
// Config is required, all other fields are optional and will use defaults if not provided.
type Options struct {
	// Config is the Contributoor configuration. This field is required.
	Config *config.Config

	// Logger is the logger to use. If nil, a default logger will be created.
	Logger logrus.FieldLogger

	// Debug enables debug mode which uses stdout sink instead of the configured output.
	Debug bool

	// ClockDrift is an optional clock drift service. If nil, one will be created.
	ClockDrift clockdrift.ClockDrift

	// NTPServer is the NTP server to use for clock drift detection.
	// Defaults to "pool.ntp.org" if not specified.
	NTPServer string

	// ClockDriftSyncInterval is the interval for syncing with NTP.
	// Defaults to 5 minutes if not specified.
	ClockDriftSyncInterval time.Duration
}

// DefaultOptions returns Options with sensible defaults.
// You must still provide a Config before creating an Application.
func DefaultOptions() Options {
	return Options{
		Logger: logrus.New().WithField("module", "contributoor"),
		Debug:  false,
	}
}

// WithConfig sets the configuration.
func (o Options) WithConfig(cfg *config.Config) Options {
	o.Config = cfg

	return o
}

// WithLogger sets the logger.
func (o Options) WithLogger(logger logrus.FieldLogger) Options {
	o.Logger = logger

	return o
}

// WithDebug enables or disables debug mode.
func (o Options) WithDebug(debug bool) Options {
	o.Debug = debug

	return o
}

// WithClockDrift sets the clock drift service.
func (o Options) WithClockDrift(cd clockdrift.ClockDrift) Options {
	o.ClockDrift = cd

	return o
}
