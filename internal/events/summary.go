package events

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// Summary is a struct that holds the summary of the sentry.
type Summary struct {
	log               logrus.FieldLogger
	traceID           string
	printInterval     time.Duration
	eventStreamEvents sync.Map
	eventsExported    atomic.Uint64
	failedEvents      atomic.Uint64
}

// topicCount is a struct that holds the topic and count of an event stream.
type topicCount struct {
	topic string
	count uint64
}

// NewSummary creates a new summary with the given print interval.
func NewSummary(log logrus.FieldLogger, traceID string, printInterval time.Duration) *Summary {
	return &Summary{
		log:           log,
		traceID:       traceID,
		printInterval: printInterval,
	}
}

// Start starts the summary ticker.
func (s *Summary) Start(ctx context.Context) {
	s.log.WithFields(logrus.Fields{
		"interval": s.printInterval,
		"trace_id": s.traceID,
	}).Info("Starting summary")

	ticker := time.NewTicker(s.printInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Print()
		}
	}
}

// Print prints the summary on interval.
func (s *Summary) Print() {
	// Build a sorted slice of event stream topics and counts.
	var (
		events       = s.GetEventStreamEvents()
		sortedEvents = make([]topicCount, 0, len(events))
	)

	for topic, count := range events {
		sortedEvents = append(sortedEvents, topicCount{topic, count})
	}

	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].count > sortedEvents[j].count
	})

	// Create formatted strings for each topic and count.
	eventTopics := make([]string, len(sortedEvents))
	for i, tc := range sortedEvents {
		eventTopics[i] = fmt.Sprintf("%s: %d", tc.topic, tc.count)
	}

	eventStream := strings.Join(eventTopics, ", ")

	s.log.WithFields(logrus.Fields{
		"trace_id":            s.traceID,
		"events_exported":     s.GetEventsExported(),
		"events_failed":       s.GetFailedEvents(),
		"event_stream_events": eventStream,
	}).Infof("Summary of the last %s for %s", s.printInterval, s.traceID)

	s.Reset()
}

// AddEventsExported adds the given count to the events exported.
func (s *Summary) AddEventsExported(count uint64) {
	s.eventsExported.Add(count)
}

// GetEventsExported returns the number of events exported.
func (s *Summary) GetEventsExported() uint64 {
	return s.eventsExported.Load()
}

// AddFailedEvents adds the given count to the failed events.
func (s *Summary) AddFailedEvents(count uint64) {
	s.failedEvents.Add(count)
}

// GetFailedEvents returns the number of failed events.
func (s *Summary) GetFailedEvents() uint64 {
	return s.failedEvents.Load()
}

// AddEventStreamEvents adds the given count to the event stream topic.
func (s *Summary) AddEventStreamEvents(topic string, count uint64) {
	value, loaded := s.eventStreamEvents.Load(topic)
	if loaded {
		currentCount, _ := value.(uint64)
		s.eventStreamEvents.Store(topic, currentCount+count)
	} else {
		s.eventStreamEvents.Store(topic, count)
	}
}

// GetEventStreamEvents returns the event stream topics and counts.
func (s *Summary) GetEventStreamEvents() map[string]uint64 {
	events := make(map[string]uint64)

	s.eventStreamEvents.Range(func(key, value any) bool {
		events[key.(string)], _ = value.(uint64)

		return true
	})

	return events
}

// Reset resets the summary.
func (s *Summary) Reset() {
	s.eventsExported.Store(0)
	s.failedEvents.Store(0)

	// Clear the map by deleting all entries.
	s.eventStreamEvents.Range(func(key, _ interface{}) bool {
		s.eventStreamEvents.Delete(key)

		return true
	})
}
