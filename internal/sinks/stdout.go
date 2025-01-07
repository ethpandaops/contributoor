package sinks

import (
	"context"
	"time"

	"github.com/creasty/defaults"
	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/ethpandaops/xatu/pkg/output"
	"github.com/ethpandaops/xatu/pkg/output/stdout"
	"github.com/ethpandaops/xatu/pkg/processor"
	pxatu "github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/sirupsen/logrus"
)

// stdoutSink is the stdout sink.
type stdoutSink struct {
	log  *logrus.Logger
	conf *stdout.Config
	sink output.Sink
}

// NewStdoutSink creates a new StdoutSink.
func NewStdoutSink(log *logrus.Logger, config *config.Config, networkName string) (ContributoorSink, error) {
	conf := &stdout.Config{}
	if err := defaults.Set(conf); err != nil {
		return nil, err
	}

	sink, err := stdout.New(networkName, conf, log, &pxatu.EventFilterConfig{}, processor.ShippingMethodAsync)
	if err != nil {
		return nil, err
	}

	return &stdoutSink{
		log:  log,
		conf: conf,
		sink: sink,
	}, nil
}

// Start starts the stdout sink.
func (s *stdoutSink) Start(ctx context.Context) error {
	s.log.WithField("type", s.sink.Type()).WithField("name", s.sink.Name()).Info("Starting sink")

	if err := s.sink.Start(ctx); err != nil {
		return err
	}

	return nil
}

// Stop stops the stdout sink.
func (s *stdoutSink) Stop(ctx context.Context) error {
	s.log.Info("Stopping stdout sink")

	return nil
}

// HandleEvent processes an event and logs it to stdout.
func (s *stdoutSink) HandleEvent(ctx context.Context, event events.Event) error {
	s.log.WithFields(logrus.Fields{
		"event_type":  event.Type(),
		"received_at": event.Time().Format(time.RFC3339),
	}).Info("Event received")

	return nil
}

func (s *stdoutSink) Name() string {
	return "stdout"
}
