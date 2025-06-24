package application

import (
	"context"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/internal/clockdrift"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockClockDrift is a mock implementation of the ClockDrift interface.
type MockClockDrift struct {
	mock.Mock
}

func (m *MockClockDrift) GetDrift() time.Duration {
	args := m.Called()

	return args.Get(0).(time.Duration) //nolint:errcheck // testing.
}

func (m *MockClockDrift) Now() time.Time {
	args := m.Called()

	return args.Get(0).(time.Time) //nolint:errcheck // testing.
}

func (m *MockClockDrift) Start(ctx context.Context) error {
	args := m.Called(ctx)

	return args.Error(0)
}

func (m *MockClockDrift) Stop(ctx context.Context) error {
	args := m.Called(ctx)

	return args.Error(0)
}

// TestInitClockDrift tests the initClockDrift method.
func TestInitClockDrift(t *testing.T) {
	tests := []struct {
		name            string
		existingService clockdrift.ClockDrift
		ntpServer       string
		syncInterval    time.Duration
		startError      error
		expectError     bool
		expectInit      bool
	}{
		{
			name:            "successful initialization",
			existingService: nil,
			ntpServer:       "pool.ntp.org",
			syncInterval:    5 * time.Minute,
			startError:      nil,
			expectError:     false,
			expectInit:      true,
		},
		{
			name:            "already initialized",
			existingService: &MockClockDrift{},
			ntpServer:       "pool.ntp.org",
			syncInterval:    5 * time.Minute,
			startError:      nil,
			expectError:     false,
			expectInit:      false,
		},
		{
			name:            "initialization with custom ntp server",
			existingService: nil,
			ntpServer:       "time.google.com",
			syncInterval:    10 * time.Minute,
			startError:      nil,
			expectError:     false,
			expectInit:      true,
		},
		{
			name:            "initialization with short sync interval",
			existingService: nil,
			ntpServer:       "pool.ntp.org",
			syncInterval:    1 * time.Minute,
			startError:      nil,
			expectError:     false,
			expectInit:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test logger
			logger := logrus.New()
			logger.SetLevel(logrus.DebugLevel)

			// Create application instance
			app := &Application{
				log:                    logger,
				clockDrift:             tt.existingService,
				ntpServer:              tt.ntpServer,
				clockDriftSyncInterval: tt.syncInterval,
			}

			// Initialize clock drift
			ctx := context.Background()
			err := app.initClockDrift(ctx)

			// Check error expectation
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Check initialization expectation
			if tt.expectInit {
				assert.NotNil(t, app.clockDrift)
				// Verify it's a real service instance
				_, ok := app.clockDrift.(*clockdrift.Service)
				assert.True(t, ok, "Expected clockDrift to be a *clockdrift.Service")
			}

			// If already initialized, verify it wasn't replaced
			if tt.existingService != nil {
				assert.Equal(t, tt.existingService, app.clockDrift)
			}
		})
	}
}

// TestClockDriftMethod tests the ClockDrift getter method.
func TestClockDriftMethod(t *testing.T) {
	tests := []struct {
		name     string
		service  clockdrift.ClockDrift
		expected clockdrift.ClockDrift
	}{
		{
			name:     "returns nil when not initialized",
			service:  nil,
			expected: nil,
		},
		{
			name:     "returns mock service",
			service:  &MockClockDrift{},
			expected: &MockClockDrift{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &Application{
				clockDrift: tt.service,
			}

			result := app.ClockDrift()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestInitClockDriftIntegration tests the integration with actual clock drift service.
func TestInitClockDriftIntegration(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a test logger
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create application instance with valid config
	app := &Application{
		log:                    logger,
		ntpServer:              "pool.ntp.org",
		clockDriftSyncInterval: 1 * time.Minute,
	}

	// Initialize clock drift
	ctx := context.Background()
	err := app.initClockDrift(ctx)
	require.NoError(t, err)
	require.NotNil(t, app.clockDrift)

	// Test the service methods
	service, ok := app.clockDrift.(*clockdrift.Service)
	require.True(t, ok)

	// Get drift should return a value (might be 0)
	drift := service.GetDrift()
	t.Logf("Current clock drift: %v", drift)

	// Now should return current time adjusted by drift
	now := service.Now()
	assert.WithinDuration(t, time.Now(), now, 10*time.Second)

	// Stop the service
	err = service.Stop(ctx)
	assert.NoError(t, err)
}

// TestClockDriftServiceMethods tests the ClockDrift interface methods.
func TestClockDriftServiceMethods(t *testing.T) {
	// Create mock clock drift
	mockClockDrift := new(MockClockDrift)

	// Test GetDrift
	expectedDrift := 100 * time.Millisecond
	mockClockDrift.On("GetDrift").Return(expectedDrift)

	drift := mockClockDrift.GetDrift()
	assert.Equal(t, expectedDrift, drift)

	// Test Now
	expectedTime := time.Now().Add(expectedDrift)
	mockClockDrift.On("Now").Return(expectedTime)

	now := mockClockDrift.Now()
	assert.Equal(t, expectedTime, now)

	// Verify all expectations were met
	mockClockDrift.AssertExpectations(t)
}

// TestClockDriftNilApplication tests handling of nil application fields.
func TestClockDriftNilApplication(t *testing.T) {
	// Test with nil logger (should panic as expected)
	app := &Application{
		ntpServer:              "pool.ntp.org",
		clockDriftSyncInterval: 5 * time.Minute,
	}

	// This should panic with nil logger
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected initClockDrift to panic with nil logger, but it didn't")
		}
	}()

	ctx := context.Background()
	// We expect this to panic
	_ = app.initClockDrift(ctx)
}
