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

	sink, err := xatu.New(networkName, conf, log, &pxatu.EventFilterConfig{}, processor.ShippingMethodAsync)
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

	if err := s.sink.Start(ctx); err != nil {
		return err
	}

	return nil
}

// Stop stops the xatu sink.
func (s *xatuSink) Stop(ctx context.Context) error {
	s.log.Info("Stopping xatu sink")

	return nil
}

// HandleEvent processes an event and forwards it to the xatu sink.
func (s *xatuSink) HandleEvent(ctx context.Context, event events.Event) error {
	return s.sink.HandleNewDecoratedEvent(ctx, event.Decorated())
}

func (s *xatuSink) Name() string {
	return "xatu"
}
