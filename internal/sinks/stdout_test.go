package sinks

import (
	"context"
	"testing"
	"time"

	"github.com/ethpandaops/bamboo/proto/contributoor/config/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdoutSink(t *testing.T) {
	log := logrus.New()
	config := &config.Config{}

	t.Run("creates new sink", func(t *testing.T) {
		sink, err := NewStdoutSink(log, config, "test-network")
		require.NoError(t, err)
		assert.NotNil(t, sink)
		assert.Equal(t, "stdout", sink.Name())
	})

	t.Run("lifecycle", func(t *testing.T) {
		sink, err := NewStdoutSink(log, config, "test-network")
		require.NoError(t, err)

		ctx := context.Background()

		// Test Start
		err = sink.Start(ctx)
		assert.NoError(t, err)

		// Test HandleEvent
		event := &mockEvent{
			eventType: "test_event",
			time:      time.Now(),
			decorated: &xatu.DecoratedEvent{
				Meta: &xatu.Meta{},
			},
		}
		err = sink.HandleEvent(ctx, event)
		assert.NoError(t, err)

		// Test Stop
		err = sink.Stop(ctx)
		assert.NoError(t, err)
	})
}
