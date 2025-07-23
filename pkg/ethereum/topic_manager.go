package ethereum

import (
	"context"
	"sync"
	"time"

	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/sirupsen/logrus"
)

//go:generate mockgen -package mock -destination mock/topic_manager.mock.go github.com/ethpandaops/contributoor/pkg/ethereum TopicManager

// TopicCondition is a function that determines if a topic should be subscribed to.
type TopicCondition func(ctx context.Context) (bool, error)

// TopicManager manages topic subscriptions and tracks attestation subnets.
type TopicManager interface {
	RegisterCondition(topic string, condition TopicCondition)
	ShouldSubscribe(ctx context.Context, topic string) bool
	GetEnabledTopics(ctx context.Context) []string
	ExcludeTopic(topic string)
	IsExcluded(topic string) bool
	SetAdvertisedSubnets(subnets []int)
	RecordAttestation(subnetID uint64, slot phase0.Slot)
	IsActiveSubnet(subnetID uint64) bool
	NeedsReconnection() <-chan struct{}
	ResetAfterReconnection()
	GetCooldownPeriod() time.Duration
	StartSubnetRefresh(ctx context.Context, refreshInterval time.Duration, nodeIdentityFetcher func() []int)
	StopSubnetRefresh()
}

// TopicConfig configures topic manager behavior including subnet tracking.
type TopicConfig struct {
	// AllTopics
	AllTopics []string
	// OptInTopics
	OptInTopics []string
	// AttestationEnabled controls if attestation subnet checking is enabled.
	AttestationEnabled bool
	// AttestationMaxSubnets is the max subnets before disabling single_attestation.
	AttestationMaxSubnets int
	// MismatchDetectionWindow is the number of slots to track.
	MismatchDetectionWindow int
	// MismatchThreshold is the number of mismatches before reconnection.
	MismatchThreshold int
	// MismatchCooldown is the cooldown period between reconnections.
	MismatchCooldown time.Duration
	// SubnetHighWaterMark is the threshold for temporary subnet participation.
	SubnetHighWaterMark int
}

// topicManager implements TopicManager with attestation subnet tracking.
type topicManager struct {
	mu  sync.RWMutex
	log logrus.FieldLogger

	// Topic management.
	allTopics      []string
	optInTopics    map[string]bool
	conditions     map[string]TopicCondition
	excludedTopics map[string]bool

	// Attestation subnet tracking.
	advertisedSubnets []int
	seenSubnets       map[uint64]bool
	trackingStartSlot phase0.Slot

	// Configuration.
	detectionWindow   int
	mismatchThreshold int
	cooldownPeriod    time.Duration
	highWaterMark     int

	// Mismatch tracking.
	mismatchCount    int
	lastMismatchTime time.Time
	mismatchEnabled  bool

	// Reconnection signaling.
	reconnectChan chan struct{}
	reconnectOnce sync.Once

	// Subnet refresh.
	refreshCancel context.CancelFunc
}

// NewTopicManager creates a new topic manager with subnet configuration.
func NewTopicManager(log logrus.FieldLogger, config *TopicConfig) TopicManager {
	cfg := getDefaultTopicConfig()
	if config != nil {
		cfg = config
	}

	optInMap := make(map[string]bool, len(cfg.OptInTopics))
	for _, topic := range cfg.OptInTopics {
		optInMap[topic] = true
	}

	return &topicManager{
		log:               log.WithField("component", "topic_manager"),
		allTopics:         cfg.AllTopics,
		conditions:        make(map[string]TopicCondition),
		optInTopics:       optInMap,
		excludedTopics:    make(map[string]bool),
		seenSubnets:       make(map[uint64]bool),
		detectionWindow:   cfg.MismatchDetectionWindow,
		mismatchThreshold: cfg.MismatchThreshold,
		cooldownPeriod:    cfg.MismatchCooldown,
		highWaterMark:     cfg.SubnetHighWaterMark,
		mismatchEnabled:   cfg.AttestationEnabled, // Mismatch detection is enabled when attestation is enabled
		reconnectChan:     make(chan struct{}),
	}
}

// RegisterCondition registers a condition for a specific topic.
func (tm *topicManager) RegisterCondition(topic string, condition TopicCondition) {
	tm.conditions[topic] = condition
}

// ShouldSubscribe checks if a topic should be subscribed to.
func (tm *topicManager) ShouldSubscribe(ctx context.Context, topic string) bool {
	condition, exists := tm.conditions[topic]
	if !exists {
		// Check if this is an opt-in topic
		if tm.optInTopics[topic] {
			// Opt-in topics require a condition to be included
			return false
		}
		// Regular topics are included by default
		return true
	}

	subscribe, err := condition(ctx)
	if err != nil {
		tm.log.WithError(err).WithField("topic", topic).Warn(
			"Failed to evaluate topic condition, excluding topic",
		)

		return false
	}

	return subscribe
}

// GetEnabledTopics returns only the topics that should be subscribed to.
func (tm *topicManager) GetEnabledTopics(ctx context.Context) []string {
	var enabled []string

	for _, topic := range tm.allTopics {
		// Check if topic has been explicitly excluded
		if tm.IsExcluded(topic) {
			tm.log.WithField("topic", topic).Debug("Excluding explicitly excluded topic")

			continue
		}

		if tm.ShouldSubscribe(ctx, topic) {
			enabled = append(enabled, topic)

			continue
		}

		tm.log.WithField("topic", topic).Debug("Excluding topic based on condition")
	}

	tm.log.WithField("topics", enabled).Info("Enabled subscription topics")

	return enabled
}

// ExcludeTopic marks a topic to be excluded from subscriptions.
func (tm *topicManager) ExcludeTopic(topic string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.excludedTopics[topic] = true
}

// IsExcluded checks if a topic has been excluded.
func (tm *topicManager) IsExcluded(topic string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.excludedTopics[topic]
}

// SetAdvertisedSubnets sets the subnets this node advertises for attestations.
func (tm *topicManager) SetAdvertisedSubnets(subnets []int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.advertisedSubnets = subnets
	tm.log.WithField("subnets", subnets).Info("Set advertised attestation subnets")
}

// RecordAttestation records an attestation for subnet tracking.
func (tm *topicManager) RecordAttestation(subnetID uint64, slot phase0.Slot) {
	if !tm.mismatchEnabled {
		return
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Stop processing if we've already signaled reconnection
	select {
	case <-tm.reconnectChan:
		// Channel is closed, we've already signaled reconnection
		return
	default:
	}

	// Initialize tracking window.
	if tm.trackingStartSlot == 0 {
		tm.trackingStartSlot = slot
	}

	// Reset tracking window if needed.
	if int(slot-tm.trackingStartSlot) >= tm.detectionWindow { //nolint:gosec // conversion fine.
		tm.seenSubnets = make(map[uint64]bool)
		tm.trackingStartSlot = slot
		tm.mismatchCount = 0
	}

	// Record subnet
	tm.seenSubnets[subnetID] = true

	// Check for mismatch
	if tm.checkForMismatch() {
		tm.mismatchCount++
		tm.log.WithFields(logrus.Fields{
			"mismatch_count":     tm.mismatchCount,
			"threshold":          tm.mismatchThreshold,
			"advertised_subnets": tm.advertisedSubnets,
			"seen_subnets":       tm.getSeenSubnetsList(),
			"slot":               slot,
		}).Warn("Subnet mismatch detected")

		if tm.mismatchCount >= tm.mismatchThreshold {
			// Check cooldown
			if time.Since(tm.lastMismatchTime) >= tm.cooldownPeriod {
				tm.lastMismatchTime = time.Now()
				tm.signalReconnection()
			}
		}
	}
}

// IsActiveSubnet checks if a subnet is actively subscribed.
func (tm *topicManager) IsActiveSubnet(subnetID uint64) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, subnet := range tm.advertisedSubnets {
		if uint64(subnet) == subnetID { //nolint:gosec // conversion safe.
			return true
		}
	}

	return false
}

// NeedsReconnection returns a channel that signals when reconnection is needed.
func (tm *topicManager) NeedsReconnection() <-chan struct{} {
	return tm.reconnectChan
}

// ResetAfterReconnection resets the reconnection state.
func (tm *topicManager) ResetAfterReconnection() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Reset tracking
	tm.mismatchCount = 0
	tm.seenSubnets = make(map[uint64]bool)
	tm.trackingStartSlot = 0

	// Create new channel for future reconnections
	tm.reconnectChan = make(chan struct{})
	tm.reconnectOnce = sync.Once{}
}

// GetCooldownPeriod returns the configured cooldown period between reconnections.
func (tm *topicManager) GetCooldownPeriod() time.Duration {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return tm.cooldownPeriod
}

// checkForMismatch checks if there's a subnet mismatch (must be called with lock held).
func (tm *topicManager) checkForMismatch() bool {
	// If no advertised subnets, can't have a mismatch
	if len(tm.advertisedSubnets) == 0 {
		return false
	}

	// Count non-advertised subnets from existing seenSubnets map
	nonAdvertisedCount := 0

	for seenSubnet := range tm.seenSubnets {
		isAdvertised := false

		for _, advertised := range tm.advertisedSubnets {
			if uint64(advertised) == seenSubnet { //nolint:gosec // conversion safe.
				isAdvertised = true

				break
			}
		}

		if !isAdvertised {
			nonAdvertisedCount++
		}
	}

	// Log warning when approaching high water mark (80% threshold)
	warningThreshold := int(float64(tm.highWaterMark) * 0.8)
	if nonAdvertisedCount >= warningThreshold && nonAdvertisedCount <= tm.highWaterMark {
		tm.log.WithFields(logrus.Fields{
			"non_advertised_count": nonAdvertisedCount,
			"high_water_mark":      tm.highWaterMark,
			"advertised_subnets":   tm.advertisedSubnets,
		}).Warn("Approaching subnet high water mark threshold")
	}

	// Only mismatch if we exceed high water mark
	return nonAdvertisedCount > tm.highWaterMark
}

// signalReconnection signals that a reconnection is needed.
func (tm *topicManager) signalReconnection() {
	tm.reconnectOnce.Do(func() {
		close(tm.reconnectChan)
	})
}

// getSeenSubnetsList returns a list of seen subnets for logging.
func (tm *topicManager) getSeenSubnetsList() []uint64 {
	subnets := make([]uint64, 0, len(tm.seenSubnets))
	for subnet := range tm.seenSubnets {
		subnets = append(subnets, subnet)
	}

	return subnets
}

// StartSubnetRefresh starts periodic refresh of advertised subnets.
func (tm *topicManager) StartSubnetRefresh(ctx context.Context, refreshInterval time.Duration, nodeIdentityFetcher func() []int) {
	tm.mu.Lock()
	// Cancel any existing refresh goroutine
	if tm.refreshCancel != nil {
		tm.refreshCancel()
	}

	// Create new context for this refresh goroutine
	refreshCtx, cancel := context.WithCancel(ctx)
	tm.refreshCancel = cancel
	tm.mu.Unlock()

	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-refreshCtx.Done():
				return
			case <-ticker.C:
				// Fetch current subnets
				newSubnets := nodeIdentityFetcher()
				if newSubnets != nil {
					// Check if subnets have changed
					tm.mu.Lock()
					changed := false

					if len(newSubnets) != len(tm.advertisedSubnets) {
						changed = true
					} else {
						for i, subnet := range newSubnets {
							if i >= len(tm.advertisedSubnets) || subnet != tm.advertisedSubnets[i] {
								changed = true

								break
							}
						}
					}

					if changed {
						tm.log.WithFields(logrus.Fields{
							"old_subnets": tm.advertisedSubnets,
							"new_subnets": newSubnets,
						}).Info("Advertised subnets changed, updating")

						tm.advertisedSubnets = newSubnets
					}

					tm.mu.Unlock()
				}
			}
		}
	}()
}

// StopSubnetRefresh stops the periodic refresh of advertised subnets.
func (tm *topicManager) StopSubnetRefresh() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.refreshCancel != nil {
		tm.refreshCancel()
		tm.refreshCancel = nil
	}
}

// getDefaultTopicConfig returns a TopicConfig with default values.
func getDefaultTopicConfig() *TopicConfig {
	return &TopicConfig{
		AllTopics:               []string{},
		OptInTopics:             []string{},
		AttestationEnabled:      false,
		AttestationMaxSubnets:   0,
		MismatchDetectionWindow: 32,
		MismatchThreshold:       3,
		MismatchCooldown:        5 * time.Minute,
		SubnetHighWaterMark:     5,
	}
}

// CreateAttestationSubnetCondition creates a condition based on attestation subnet participation.
func CreateAttestationSubnetCondition(nodeSubnetCount int, maxSubnets int) TopicCondition {
	return func(ctx context.Context) (bool, error) {
		return nodeSubnetCount <= maxSubnets, nil
	}
}
