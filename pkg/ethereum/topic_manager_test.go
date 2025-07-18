package ethereum

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopicManager_ShouldSubscribe(t *testing.T) {
	tests := []struct {
		name            string
		topic           string
		condition       TopicCondition
		isOptIn         bool
		expectSubscribe bool
	}{
		{
			name:            "no condition registered - should subscribe",
			topic:           "block",
			condition:       nil,
			isOptIn:         false,
			expectSubscribe: true,
		},
		{
			name:            "opt-in topic with no condition - should NOT subscribe",
			topic:           "single_attestation",
			condition:       nil,
			isOptIn:         true,
			expectSubscribe: false,
		},
		{
			name:  "opt-in topic with condition returning true",
			topic: "single_attestation",
			condition: func(ctx context.Context) (bool, error) {
				return true, nil
			},
			isOptIn:         true,
			expectSubscribe: true,
		},
		{
			name:  "condition returns false",
			topic: "single_attestation",
			condition: func(ctx context.Context) (bool, error) {
				return false, nil
			},
			isOptIn:         false,
			expectSubscribe: false,
		},
		{
			name:  "condition returns error - should not subscribe",
			topic: "single_attestation",
			condition: func(ctx context.Context) (bool, error) {
				return false, errors.New("test error")
			},
			isOptIn:         false,
			expectSubscribe: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logrus.New()
			log.SetLevel(logrus.DebugLevel)

			// Use a simple topic list for testing
			allTopics := []string{"block", "head", tt.topic}
			var optInTopics []string
			if tt.isOptIn {
				optInTopics = []string{tt.topic}
			}
			tm := NewTopicManager(log, allTopics, optInTopics)

			if tt.condition != nil {
				tm.RegisterCondition(tt.topic, tt.condition)
			}

			shouldSubscribe := tm.ShouldSubscribe(context.Background(), tt.topic)
			assert.Equal(t, tt.expectSubscribe, shouldSubscribe)
		})
	}
}

func TestTopicManager_GetEnabledTopics(t *testing.T) {
	tests := []struct {
		name           string
		allTopics      []string
		conditions     map[string]TopicCondition
		expectedTopics []string
		setupOptIn     bool
	}{
		{
			name:           "no conditions - all topics included",
			allTopics:      []string{"block", "head", "single_attestation"},
			conditions:     map[string]TopicCondition{},
			expectedTopics: []string{"block", "head", "single_attestation"},
		},
		{
			name:      "filter out single_attestation",
			allTopics: []string{"block", "head", "single_attestation"},
			conditions: map[string]TopicCondition{
				"single_attestation": func(ctx context.Context) (bool, error) {
					return false, nil
				},
			},
			expectedTopics: []string{"block", "head"},
		},
		{
			name:      "multiple conditions",
			allTopics: []string{"block", "head", "single_attestation", "blob_sidecar"},
			conditions: map[string]TopicCondition{
				"single_attestation": func(ctx context.Context) (bool, error) {
					return false, nil
				},
				"head": func(ctx context.Context) (bool, error) {
					return false, nil
				},
			},
			expectedTopics: []string{"block", "blob_sidecar"},
		},
		{
			name:      "condition with error excludes topic",
			allTopics: []string{"block", "head", "single_attestation"},
			conditions: map[string]TopicCondition{
				"head": func(ctx context.Context) (bool, error) {
					return true, errors.New("test error")
				},
			},
			expectedTopics: []string{"block", "single_attestation"},
		},
		{
			name:           "opt-in topic without condition is excluded",
			allTopics:      []string{"block", "head", "single_attestation"},
			conditions:     map[string]TopicCondition{},
			expectedTopics: []string{"block", "head"},
			setupOptIn:     true, // Will mark single_attestation as opt-in
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logrus.New()
			log.SetLevel(logrus.DebugLevel)

			var optInTopics []string
			if tt.setupOptIn {
				optInTopics = []string{"single_attestation"}
			}
			tm := NewTopicManager(log, tt.allTopics, optInTopics)

			for topic, condition := range tt.conditions {
				tm.RegisterCondition(topic, condition)
			}

			enabledTopics := tm.GetEnabledTopics(context.Background())
			assert.Equal(t, tt.expectedTopics, enabledTopics)
		})
	}
}

func TestCreateAttestationSubnetCondition(t *testing.T) {
	// This test would need to mock the HTTP request
	// For now, just test that the condition function is created
	log := logrus.New()
	condition := CreateAttestationSubnetCondition(log, "http://localhost:5052", nil, 2)

	require.NotNil(t, condition)
	// The actual condition test would require mocking HTTP responses
}
