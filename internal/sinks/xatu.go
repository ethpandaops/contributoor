package sinks

import (
	"context"
	"fmt"

	"github.com/creasty/defaults"
	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/ethpandaops/xatu/pkg/output"
	"github.com/ethpandaops/xatu/pkg/output/xatu"
	"github.com/ethpandaops/xatu/pkg/processor"
	pxatu "github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/sirupsen/logrus"
)

// xatuSink is the xatu sink.
type xatuSink struct {
	log  logrus.FieldLogger
	conf *xatu.Config
	sink output.Sink

	// runCancel stops the context given to the underlying processor's workers, which is
	// deliberately independent of whatever context the caller passes to Start. The worker
	// goroutines hold onto that context for their entire lifetime, including any export attempts
	// made while draining on shutdown, so it must not be tied to the application's SIGTERM-
	// cancellable context or a drain would be exporting into an already-cancelled context.
	runCancel context.CancelFunc
}

// NewXatuSink creates a new XatuSink.
func NewXatuSink(log logrus.FieldLogger, config *config.Config, networkName string) (ContributoorSink, error) {
	conf := &xatu.Config{}
	if err := defaults.Set(conf); err != nil {
		return nil, err
	}

	conf.TLS = config.OutputServer.Tls
	conf.Address = config.OutputServer.Address

	if config.OutputServer.Credentials != "" {
		conf.Headers = map[string]string{
			"authorization": fmt.Sprintf("Basic %s", config.OutputServer.Credentials),
		}
	}

	sink, err := xatu.New(networkName, conf, log.WithField("sink", "xatu"), &pxatu.EventFilterConfig{}, processor.ShippingMethodAsync)
	if err != nil {
		return nil, err
	}

	return &xatuSink{
		log:  log,
		conf: conf,
		sink: sink,
	}, nil
}

// Start starts the xatu sink.
func (s *xatuSink) Start(ctx context.Context) error {
	s.log.WithField("type", s.sink.Type()).WithField("name", s.sink.Name()).Debug("Starting sink")

	// The underlying processor's workers run for as long as this context lives, so it must
	// outlive the caller's context (which is cancelled on shutdown, before Stop is even called).
	var runCtx context.Context

	runCtx, s.runCancel = context.WithCancel(context.Background())

	if err := s.sink.Start(runCtx); err != nil {
		s.runCancel()

		return err
	}

	return nil
}

// Stop stops the xatu sink, draining any buffered or in-flight events before returning.
func (s *xatuSink) Stop(ctx context.Context) error {
	s.log.Info("Stopping xatu sink")

	if s.runCancel == nil {
		// Stop called without a prior successful Start; nothing to drain.
		return nil
	}

	defer s.runCancel()

	if err := s.sink.Stop(ctx); err != nil {
		if ctx.Err() != nil {
			s.log.WithError(err).Warn(
				"Xatu sink did not finish draining before shutdown deadline; some buffered " +
					"events may not have been delivered",
			)
		}

		return err
	}

	return nil
}

// HandleEvent processes an event and forwards it to the xatu sink.
func (s *xatuSink) HandleEvent(ctx context.Context, event events.Event) error {
	return s.sink.HandleNewDecoratedEvent(ctx, event.Decorated())
}

func (s *xatuSink) Name() string {
	return "xatu"
}
