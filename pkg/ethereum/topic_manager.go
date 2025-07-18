package ethereum

import (
	"context"

	"github.com/sirupsen/logrus"
)

// TopicCondition is a function that determines if a topic should be subscribed to.
type TopicCondition func(ctx context.Context) (bool, error)

// TopicManager manages conditional topic subscriptions.
type TopicManager struct {
	log        logrus.FieldLogger
	conditions map[string]TopicCondition
}

// NewTopicManager creates a new topic manager.
func NewTopicManager(log logrus.FieldLogger) *TopicManager {
	return &TopicManager{
		log:        log.WithField("component", "topic_manager"),
		conditions: make(map[string]TopicCondition),
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
		// No condition registered, subscribe by default
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

// FilterTopics returns only the topics that should be subscribed to.
func (tm *TopicManager) FilterTopics(ctx context.Context, topics []string) []string {
	var filtered []string

	for _, topic := range topics {
		if tm.ShouldSubscribe(ctx, topic) {
			filtered = append(filtered, topic)

			continue
		}

		tm.log.WithField("topic", topic).Debug("Excluding topic based on condition")
	}

	tm.log.WithField("topics", filtered).Info("Filtered subscription topics")

	return filtered
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
