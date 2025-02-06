package events

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewSummary(t *testing.T) {
	log := logrus.New()
	interval := 5 * time.Second

	summary := NewSummary(log, "test-trace-id", interval)
	assert.NotNil(t, summary)
	assert.Equal(t, interval, summary.printInterval)
}

func TestSummary_EventsExported(t *testing.T) {
	summary := NewSummary(logrus.New(), "test-trace-id", time.Second)

	// Test adding and getting events exported
	summary.AddEventsExported(5)
	assert.Equal(t, uint64(5), summary.GetEventsExported())

	summary.AddEventsExported(3)
	assert.Equal(t, uint64(8), summary.GetEventsExported())
}

func TestSummary_FailedEvents(t *testing.T) {
	summary := NewSummary(logrus.New(), "test-trace-id", time.Second)

	// Test adding and getting failed events
	summary.AddFailedEvents(2)
	assert.Equal(t, uint64(2), summary.GetFailedEvents())

	summary.AddFailedEvents(1)
	assert.Equal(t, uint64(3), summary.GetFailedEvents())
}

func TestSummary_EventStreamEvents(t *testing.T) {
	summary := NewSummary(logrus.New(), "test-trace-id", time.Second)

	// Test adding and getting event stream events
	summary.AddEventStreamEvents("topic1", 3)
	summary.AddEventStreamEvents("topic2", 2)
	summary.AddEventStreamEvents("topic1", 1)

	events := summary.GetEventStreamEvents()
	assert.Equal(t, uint64(4), events["topic1"])
	assert.Equal(t, uint64(2), events["topic2"])
}

func TestSummary_Reset(t *testing.T) {
	summary := NewSummary(logrus.New(), "test-trace-id", time.Second)

	// Add some data
	summary.AddEventsExported(5)
	summary.AddFailedEvents(2)
	summary.AddEventStreamEvents("topic1", 3)

	// Reset and verify everything is cleared
	summary.Reset()

	assert.Equal(t, uint64(0), summary.GetEventsExported())
	assert.Equal(t, uint64(0), summary.GetFailedEvents())
	assert.Empty(t, summary.GetEventStreamEvents())
}

func TestSummary_Start(t *testing.T) {
	summary := NewSummary(logrus.New(), "test-trace-id", 100*time.Millisecond)

	// Add some test data
	summary.AddEventsExported(5)
	summary.AddFailedEvents(2)
	summary.AddEventStreamEvents("topic1", 3)

	// Start the summary in a goroutine
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	go summary.Start(ctx)

	// Wait for at least two print intervals
	time.Sleep(220 * time.Millisecond)

	// Verify that the summary was reset after printing
	events := summary.GetEventStreamEvents()
	assert.Empty(t, events)
	assert.Equal(t, uint64(0), summary.GetEventsExported())
	assert.Equal(t, uint64(0), summary.GetFailedEvents())
}

func TestSummary_Print(t *testing.T) {
	summary := NewSummary(logrus.New(), "test-trace-id", time.Second)

	// Add test data
	summary.AddEventsExported(10)
	summary.AddFailedEvents(2)
	summary.AddEventStreamEvents("topic1", 5)
	summary.AddEventStreamEvents("topic2", 3)

	// Call Print and verify it doesn't panic
	summary.Print()

	// Verify reset was called
	assert.Equal(t, uint64(0), summary.GetEventsExported())
	assert.Equal(t, uint64(0), summary.GetFailedEvents())
	assert.Empty(t, summary.GetEventStreamEvents())
}
