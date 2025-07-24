package ethereum

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestIsSlotDifferenceTooLarge(t *testing.T) {
	tests := []struct {
		name           string
		slotA          uint64
		slotB          uint64
		expectTooLarge bool
	}{
		{
			name:           "identical slots",
			slotA:          100,
			slotB:          100,
			expectTooLarge: false,
		},
		{
			name:           "small difference (positive direction)",
			slotA:          100,
			slotB:          1000, // 900 slot difference
			expectTooLarge: false,
		},
		{
			name:           "small difference (negative direction)",
			slotA:          1000,
			slotB:          100, // 900 slot difference
			expectTooLarge: false,
		},
		{
			name:           "at threshold",
			slotA:          100,
			slotB:          10100, // exactly 10000 slot difference
			expectTooLarge: false,
		},
		{
			name:           "beyond threshold (positive direction)",
			slotA:          100,
			slotB:          12000, // 11900 slot difference, > MaxReasonableSlotDifference (10000)
			expectTooLarge: true,
		},
		{
			name:           "beyond threshold (negative direction)",
			slotA:          12000,
			slotB:          100, // 11900 slot difference, > MaxReasonableSlotDifference (10000)
			expectTooLarge: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSlotDifferenceTooLarge(tt.slotA, tt.slotB)
			assert.Equal(t, tt.expectTooLarge, result)
		})
	}
}

func TestBeaconWrapper_IsActiveSubnet(t *testing.T) {
	// Test 1: Empty active subnets denies all
	t.Run("empty active subnets denies all", func(t *testing.T) {
		log := logrus.New()
		topicConfig := &TopicConfig{
			AllTopics:             GetDefaultAllTopics(),
			OptInTopics:           GetOptInTopics(),
			AttestationEnabled:    true,
			AttestationMaxSubnets: 64,
		}
		topicMgr := NewTopicManager(log, topicConfig)
		topicMgr.SetAdvertisedSubnets([]int{})

		w := &BeaconWrapper{topicManager: topicMgr}
		assert.False(t, w.IsActiveSubnet(5))
	})

	// Test 2: With advertised subnets, only one is randomly selected
	t.Run("only one subnet from advertised list is active", func(t *testing.T) {
		log := logrus.New()
		topicConfig := &TopicConfig{
			AllTopics:             GetDefaultAllTopics(),
			OptInTopics:           GetOptInTopics(),
			AttestationEnabled:    true,
			AttestationMaxSubnets: 64,
		}
		topicMgr := NewTopicManager(log, topicConfig)
		advertisedSubnets := []int{2, 5, 10}
		topicMgr.SetAdvertisedSubnets(advertisedSubnets)

		w := &BeaconWrapper{topicManager: topicMgr}

		// Count how many subnets are active
		activeCount := 0
		var activeSubnet int
		for _, subnet := range advertisedSubnets {
			if w.IsActiveSubnet(uint64(subnet)) {
				activeCount++
				activeSubnet = subnet
			}
		}

		assert.Equal(t, 1, activeCount, "Exactly one subnet should be active")
		assert.Contains(t, advertisedSubnets, activeSubnet, "Active subnet should be from advertised list")

		// Non-advertised subnet should not be active
		assert.False(t, w.IsActiveSubnet(7))
	})

	// Test 3: Single subnet is always selected
	t.Run("single subnet is always selected", func(t *testing.T) {
		log := logrus.New()
		topicConfig := &TopicConfig{
			AllTopics:             GetDefaultAllTopics(),
			OptInTopics:           GetOptInTopics(),
			AttestationEnabled:    true,
			AttestationMaxSubnets: 64,
		}
		topicMgr := NewTopicManager(log, topicConfig)
		topicMgr.SetAdvertisedSubnets([]int{63})

		w := &BeaconWrapper{topicManager: topicMgr}
		assert.True(t, w.IsActiveSubnet(63))
		assert.False(t, w.IsActiveSubnet(62))
	})
}

func TestCalculateSubnetID(t *testing.T) {
	tests := []struct {
		committeeIndex uint64
		expectedSubnet uint64
	}{
		{0, 0},
		{1, 1},
		{63, 63},
		{64, 0},
		{65, 1},
		{127, 63},
		{128, 0},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			subnetID := tt.committeeIndex % 64
			assert.Equal(t, tt.expectedSubnet, subnetID)
		})
	}
}
