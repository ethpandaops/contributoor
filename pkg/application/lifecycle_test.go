package application

import (
	"context"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/pkg/ethereum/mock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestApplicationStop_NilContextAppliesBoundedDeadline locks in that passing a nil context to
// Stop (as cmd/sentry/main.go now does) actually results in every downstream Stop call receiving
// a bounded-deadline context, rather than the unbounded context.Background() that was previously
// passed explicitly. Without this, a hung sink drain could block shutdown indefinitely.
func TestApplicationStop_NilContextAppliesBoundedDeadline(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNode := mock.NewMockBeaconNodeAPI(ctrl)

	var capturedCtx context.Context

	mockNode.EXPECT().Stop(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		capturedCtx = ctx

		return nil
	})

	app := &Application{
		log:     logrus.New(),
		servers: &ServerManager{},
		beaconNodes: map[string]*BeaconNodeInstance{
			"trace-a": {Node: mockNode},
		},
	}

	require.NoError(t, app.Stop(nil)) //nolint:staticcheck // nil is intentional, this is exactly what main.go now passes.

	require.NotNil(t, capturedCtx, "instance.Node.Stop must have been called")

	deadline, ok := capturedCtx.Deadline()
	require.True(t, ok, "a nil ctx passed to Stop must result in downstream calls receiving a "+
		"context with a deadline, not an unbounded one")

	assert.WithinDuration(t, time.Now().Add(15*time.Second), deadline, 2*time.Second,
		"the deadline should match Stop's own 15 second shutdown timeout")
}

// TestApplicationStop_ExplicitContextIsRespected confirms Stop still honors a caller-supplied
// context instead of always substituting its own default, so callers that want their own
// deadline (or an unbounded one, deliberately) are not overridden.
func TestApplicationStop_ExplicitContextIsRespected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNode := mock.NewMockBeaconNodeAPI(ctrl)

	var capturedCtx context.Context

	mockNode.EXPECT().Stop(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		capturedCtx = ctx

		return nil
	})

	app := &Application{
		log:     logrus.New(),
		servers: &ServerManager{},
		beaconNodes: map[string]*BeaconNodeInstance{
			"trace-a": {Node: mockNode},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, app.Stop(ctx))

	deadline, ok := capturedCtx.Deadline()
	require.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(5*time.Second), deadline, time.Second,
		"Stop must pass through the caller's own context rather than replacing it")
}
