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
		expectSubscribe bool
	}{
		{
			name:            "no condition registered - should subscribe",
			topic:           "block",
			condition:       nil,
			expectSubscribe: true,
		},
		{
			name:  "condition returns true",
			topic: "single_attestation",
			condition: func(ctx context.Context) (bool, error) {
				return true, nil
			},
			expectSubscribe: true,
		},
		{
			name:  "condition returns false",
			topic: "single_attestation",
			condition: func(ctx context.Context) (bool, error) {
				return false, nil
			},
			expectSubscribe: false,
		},
		{
			name:  "condition returns error - should not subscribe",
			topic: "single_attestation",
			condition: func(ctx context.Context) (bool, error) {
				return false, errors.New("test error")
			},
			expectSubscribe: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logrus.New()
			log.SetLevel(logrus.DebugLevel)
			tm := NewTopicManager(log)

			if tt.condition != nil {
				tm.RegisterCondition(tt.topic, tt.condition)
			}

			shouldSubscribe := tm.ShouldSubscribe(context.Background(), tt.topic)
			assert.Equal(t, tt.expectSubscribe, shouldSubscribe)
		})
	}
}

func TestTopicManager_FilterTopics(t *testing.T) {
	tests := []struct {
		name           string
		allTopics      []string
		conditions     map[string]TopicCondition
		expectedTopics []string
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logrus.New()
			log.SetLevel(logrus.DebugLevel)
			tm := NewTopicManager(log)

			for topic, condition := range tt.conditions {
				tm.RegisterCondition(topic, condition)
			}

			filteredTopics := tm.FilterTopics(context.Background(), tt.allTopics)
			assert.Equal(t, tt.expectedTopics, filteredTopics)
		})
	}
}

func TestCreateSubnetCondition(t *testing.T) {
	// This test would need to mock the HTTP request
	// For now, just test that the condition function is created
	log := logrus.New()
	condition := CreateSubnetCondition(log, "http://localhost:5052", nil, 2)

	require.NotNil(t, condition)
	// The actual condition test would require mocking HTTP responses
}
