package sinks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestXatuSink(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "valid config with credentials",
			config: &config.Config{
				OutputServer: &config.OutputServer{
					Address:     "localhost:8080",
					Credentials: "test-creds",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config without credentials",
			config: &config.Config{
				OutputServer: &config.OutputServer{
					Address: "localhost:8080",
				},
			},
			wantErr: false,
		},
		{
			name: "missing address",
			config: &config.Config{
				OutputServer: &config.OutputServer{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logrus.New()
			sink, err := NewXatuSink(log, tt.config, "test-network")

			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, sink)
			assert.Equal(t, "xatu", sink.Name())

			// Test credentials if provided
			if tt.config.OutputServer.Credentials != "" {
				xatuSink, ok := sink.(*xatuSink)
				require.True(t, ok)
				assert.Equal(t, fmt.Sprintf("Basic %s", tt.config.OutputServer.Credentials), xatuSink.conf.Headers["authorization"])
			}
		})
	}

	t.Run("lifecycle", func(t *testing.T) {
		log := logrus.New()
		config := &config.Config{
			OutputServer: &config.OutputServer{
				Address: "localhost:8080",
			},
		}

		sink, err := NewXatuSink(log, config, "test-network")
		require.NoError(t, err)

		ctx := context.Background()

		// Test Start
		err = sink.Start(ctx)
		assert.NoError(t, err)

		// Test HandleEvent
		now := time.Now()
		event := &mockEvent{
			eventType: "test_event",
			time:      now,
			decorated: &xatu.DecoratedEvent{
				Meta: &xatu.Meta{
					Client: &xatu.ClientMeta{},
				},
				Event: &xatu.Event{
					Name:     xatu.Event_BEACON_API_ETH_V1_EVENTS_ATTESTATION_V2,
					DateTime: timestamppb.New(now),
					Id:       "test-id",
				},
			},
		}
		err = sink.HandleEvent(ctx, event)
		assert.NoError(t, err)

		// Test Stop
		err = sink.Stop(ctx)
		assert.NoError(t, err)
	})
}
