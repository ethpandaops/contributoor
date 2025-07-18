package ethereum

import (
	"context"

	"github.com/sirupsen/logrus"
)

// TopicCondition is a function that determines if a topic should be subscribed to.
type TopicCondition func(ctx context.Context) (bool, error)

// TopicManager manages conditional topic subscriptions.
type TopicManager struct {
	log         logrus.FieldLogger
	allTopics   []string
	conditions  map[string]TopicCondition
	optInTopics map[string]bool // Topics that require explicit opt-in
}

// NewTopicManager creates a new topic manager.
// allTopics are all available topics that can be subscribed to.
// optInTopics are topics that require explicit conditions to be included.
func NewTopicManager(log logrus.FieldLogger, allTopics []string, optInTopics []string) *TopicManager {
	optInMap := make(map[string]bool)
	for _, topic := range optInTopics {
		optInMap[topic] = true
	}

	return &TopicManager{
		log:         log.WithField("component", "topic_manager"),
		allTopics:   allTopics,
		conditions:  make(map[string]TopicCondition),
		optInTopics: optInMap,
	}
}

// RegisterCondition registers a condition for a specific topic.
func (tm *TopicManager) RegisterCondition(topic string, condition TopicCondition) {
	tm.conditions[topic] = condition
}

// ShouldSubscribe checks if a topic should be subscribed to.
func (tm *TopicManager) ShouldSubscribe(ctx context.Context, topic string) bool {
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
func (tm *TopicManager) GetEnabledTopics(ctx context.Context) []string {
	var enabled []string

	for _, topic := range tm.allTopics {
		if tm.ShouldSubscribe(ctx, topic) {
			enabled = append(enabled, topic)

			continue
		}

		tm.log.WithField("topic", topic).Debug("Excluding topic based on condition")
	}

	tm.log.WithField("topics", enabled).Info("Enabled subscription topics")

	return enabled
}

// CreateAttestationSubnetCondition creates a condition based on attestation subnet participation.
func CreateAttestationSubnetCondition(
	log logrus.FieldLogger,
	address string,
	headers map[string]string,
	maxSubnets int,
) TopicCondition {
	return func(ctx context.Context) (bool, error) {
		subnetCount, err := CheckAttestationSubnetParticipation(ctx, address, headers)
		if err != nil {
			return false, err
		}

		shouldSubscribe := subnetCount <= maxSubnets

		log.WithFields(logrus.Fields{
			"subnet_count": subnetCount,
			"max_subnets":  maxSubnets,
			"subscribe":    shouldSubscribe,
		}).Debug("Evaluated subnet condition")

		return shouldSubscribe, nil
	}
}
