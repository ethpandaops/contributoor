package sinks

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/ethpandaops/xatu/pkg/proto/xatu"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
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

// fakeAsyncOutputSink models the real xatu async sink closely enough to test xatuSink's own
// contract with it: HandleNewDecoratedEvent only enqueues, and the queued events only become
// "exported" once Stop is called (the drain). If Stop is never called, or is called against a
// context the workers can't use, the queued events never leave the queue.
type fakeAsyncOutputSink struct {
	mu          sync.Mutex
	queued      []*xatu.DecoratedEvent
	exported    int
	startCtxErr error
	stopCalled  bool
	stopDelay   time.Duration
}

func (f *fakeAsyncOutputSink) Start(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.startCtxErr = ctx.Err()

	return nil
}

func (f *fakeAsyncOutputSink) Stop(ctx context.Context) error {
	f.mu.Lock()
	f.stopCalled = true
	delay := f.stopDelay
	f.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.exported += len(f.queued)
	f.queued = nil

	return nil
}

func (f *fakeAsyncOutputSink) Type() string { return "fake-async" }
func (f *fakeAsyncOutputSink) Name() string { return "fake-async" }

func (f *fakeAsyncOutputSink) HandleNewDecoratedEvent(_ context.Context, e *xatu.DecoratedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.queued = append(f.queued, e)

	return nil
}

func (f *fakeAsyncOutputSink) HandleNewDecoratedEvents(_ context.Context, es []*xatu.DecoratedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.queued = append(f.queued, es...)

	return nil
}

func TestXatuSink_StopDrainsBufferedEvents(t *testing.T) {
	fake := &fakeAsyncOutputSink{}
	s := &xatuSink{log: logrus.New(), sink: fake}

	ctx := context.Background()
	require.NoError(t, s.Start(ctx))

	for _, id := range []string{"e1", "e2", "e3"} {
		require.NoError(t, s.HandleEvent(ctx, &mockEvent{
			eventType: "test",
			decorated: &xatu.DecoratedEvent{Event: &xatu.Event{Id: id}},
		}))
	}

	fake.mu.Lock()
	queuedBeforeStop := len(fake.queued)
	fake.mu.Unlock()
	require.Equal(t, 3, queuedBeforeStop, "events must be queued, not yet exported, before Stop")

	require.NoError(t, s.Stop(ctx))

	fake.mu.Lock()
	defer fake.mu.Unlock()

	assert.True(t, fake.stopCalled, "Stop must call through to the underlying sink's Stop")
	assert.Equal(t, 3, fake.exported, "all buffered events must be drained on Stop")
	assert.Empty(t, fake.queued)
}

func TestXatuSink_WorkersUseContextIndependentOfCaller(t *testing.T) {
	fake := &fakeAsyncOutputSink{}
	s := &xatuSink{log: logrus.New(), sink: fake}

	// The caller's context is already cancelled before Start is even called - exactly the state
	// the application's root context is in by the time shutdown begins.
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, cancelledCtx.Err())

	require.NoError(t, s.Start(cancelledCtx))

	fake.mu.Lock()
	startCtxErr := fake.startCtxErr
	fake.mu.Unlock()

	require.NoError(t, startCtxErr,
		"the underlying sink must be started with a context independent of the caller's - "+
			"otherwise a caller-cancelled context (as happens on SIGTERM) would poison every "+
			"export attempt made during a later drain")

	require.NoError(t, s.Stop(context.Background()))
}

func TestXatuSink_StopLogsWarningWhenDeadlineExceededBeforeDrainCompletes(t *testing.T) {
	fake := &fakeAsyncOutputSink{stopDelay: 200 * time.Millisecond}

	log, hook := logrustest.NewNullLogger()
	s := &xatuSink{log: log, sink: fake}

	require.NoError(t, s.Start(context.Background()))

	stopCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := s.Stop(stopCtx)
	require.Error(t, err, "Stop must surface the deadline error rather than swallowing it")

	var found bool

	for _, entry := range hook.AllEntries() {
		if entry.Level == logrus.WarnLevel {
			found = true

			break
		}
	}

	assert.True(t, found, "a deadline-exceeded drain must log a warning about undelivered events")
}

func TestXatuSink_StopWithoutStartIsSafe(t *testing.T) {
	s := &xatuSink{log: logrus.New()}

	assert.NoError(t, s.Stop(context.Background()))
}
