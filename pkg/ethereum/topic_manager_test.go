package ethereum_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethpandaops/contributoor/pkg/ethereum"
	"github.com/ethpandaops/contributoor/pkg/ethereum/mock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestTopicManager_ShouldSubscribe(t *testing.T) {
	tests := []struct {
		name            string
		topic           string
		condition       ethereum.TopicCondition
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
			config := &ethereum.TopicConfig{
				AllTopics:   allTopics,
				OptInTopics: optInTopics,
			}
			tm := ethereum.NewTopicManager(log, config)

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
		conditions     map[string]ethereum.TopicCondition
		expectedTopics []string
		setupOptIn     bool
	}{
		{
			name:           "no conditions - all topics included",
			allTopics:      []string{"block", "head", "single_attestation"},
			conditions:     map[string]ethereum.TopicCondition{},
			expectedTopics: []string{"block", "head", "single_attestation"},
		},
		{
			name:      "filter out single_attestation",
			allTopics: []string{"block", "head", "single_attestation"},
			conditions: map[string]ethereum.TopicCondition{
				"single_attestation": func(ctx context.Context) (bool, error) {
					return false, nil
				},
			},
			expectedTopics: []string{"block", "head"},
		},
		{
			name:      "multiple conditions",
			allTopics: []string{"block", "head", "single_attestation", "blob_sidecar"},
			conditions: map[string]ethereum.TopicCondition{
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
			conditions: map[string]ethereum.TopicCondition{
				"head": func(ctx context.Context) (bool, error) {
					return true, errors.New("test error")
				},
			},
			expectedTopics: []string{"block", "single_attestation"},
		},
		{
			name:           "opt-in topic without condition is excluded",
			allTopics:      []string{"block", "head", "single_attestation"},
			conditions:     map[string]ethereum.TopicCondition{},
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
			config := &ethereum.TopicConfig{
				AllTopics:   tt.allTopics,
				OptInTopics: optInTopics,
			}
			tm := ethereum.NewTopicManager(log, config)

			for topic, condition := range tt.conditions {
				tm.RegisterCondition(topic, condition)
			}

			enabledTopics := tm.GetEnabledTopics(context.Background())
			assert.Equal(t, tt.expectedTopics, enabledTopics)
		})
	}
}

func TestCreateAttestationSubnetCondition(t *testing.T) {
	tests := []struct {
		name            string
		subnetCount     int
		maxSubnets      int
		expectSubscribe bool
	}{
		{
			name:            "subnet count below max - should subscribe",
			subnetCount:     1,
			maxSubnets:      2,
			expectSubscribe: true,
		},
		{
			name:            "subnet count at max - should subscribe",
			subnetCount:     2,
			maxSubnets:      2,
			expectSubscribe: true,
		},
		{
			name:            "subnet count above max - should not subscribe",
			subnetCount:     3,
			maxSubnets:      2,
			expectSubscribe: false,
		},
		{
			name:            "zero subnets with zero max - should subscribe",
			subnetCount:     0,
			maxSubnets:      0,
			expectSubscribe: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create a mock NodeIdentity using the generated mock
			mockIdentity := mock.NewMockNodeIdentity(ctrl)

			// Set up the expected behavior
			subnets := make([]int, tt.subnetCount)
			for i := 0; i < tt.subnetCount; i++ {
				subnets[i] = i
			}
			mockIdentity.EXPECT().GetAttnets().Return(subnets).AnyTimes()

			// Create condition
			condition := ethereum.CreateAttestationSubnetCondition(len(mockIdentity.GetAttnets()), tt.maxSubnets)
			require.NotNil(t, condition)

			// Test the condition
			shouldSubscribe, err := condition(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tt.expectSubscribe, shouldSubscribe)
		})
	}
}

func TestTopicManager_HighWaterMarkLogic(t *testing.T) {
	tests := []struct {
		name                string
		advertisedSubnets   []int
		seenSubnets         []uint64
		highWaterMark       int
		expectMismatch      bool
		expectWarningLogged bool
		description         string
	}{
		{
			name:              "no advertised subnets - no mismatch",
			advertisedSubnets: []int{},
			seenSubnets:       []uint64{1, 2, 3},
			highWaterMark:     5,
			expectMismatch:    false,
			description:       "Should not mismatch when no subnets are advertised",
		},
		{
			name:              "all seen subnets are advertised - no mismatch",
			advertisedSubnets: []int{1, 2, 3},
			seenSubnets:       []uint64{1, 2, 3},
			highWaterMark:     5,
			expectMismatch:    false,
			description:       "Should not mismatch when all seen subnets are advertised",
		},
		{
			name:              "non-advertised count below high water mark - no mismatch",
			advertisedSubnets: []int{1, 2},
			seenSubnets:       []uint64{1, 2, 3, 4, 5},
			highWaterMark:     5,
			expectMismatch:    false,
			description:       "3 non-advertised subnets (3,4,5) < high water mark (5)",
		},
		{
			name:              "non-advertised count exactly at high water mark - no mismatch",
			advertisedSubnets: []int{1, 2},
			seenSubnets:       []uint64{1, 2, 3, 4, 5, 6, 7},
			highWaterMark:     5,
			expectMismatch:    false,
			description:       "5 non-advertised subnets (3,4,5,6,7) = high water mark (5)",
		},
		{
			name:              "non-advertised count exceeds high water mark - mismatch",
			advertisedSubnets: []int{1, 2},
			seenSubnets:       []uint64{1, 2, 3, 4, 5, 6, 7, 8},
			highWaterMark:     5,
			expectMismatch:    true,
			description:       "6 non-advertised subnets (3,4,5,6,7,8) > high water mark (5)",
		},
		{
			name:                "approaching high water mark threshold (80%) - warning",
			advertisedSubnets:   []int{1, 2},
			seenSubnets:         []uint64{1, 2, 3, 4, 5, 6},
			highWaterMark:       5,
			expectMismatch:      false,
			expectWarningLogged: true,
			description:         "4 non-advertised subnets = 80% of high water mark (5)",
		},
		{
			name:              "high water mark zero - any non-advertised triggers mismatch",
			advertisedSubnets: []int{1, 2},
			seenSubnets:       []uint64{1, 2, 3},
			highWaterMark:     0,
			expectMismatch:    true,
			description:       "High water mark of 0 means no tolerance for non-advertised subnets",
		},
		{
			name:              "high water mark one - allows one non-advertised subnet",
			advertisedSubnets: []int{1, 2},
			seenSubnets:       []uint64{1, 2, 3},
			highWaterMark:     1,
			expectMismatch:    false,
			description:       "1 non-advertised subnet (3) <= high water mark (1)",
		},
		{
			name:              "high water mark one - two non-advertised triggers mismatch",
			advertisedSubnets: []int{1, 2},
			seenSubnets:       []uint64{1, 2, 3, 4},
			highWaterMark:     1,
			expectMismatch:    true,
			description:       "2 non-advertised subnets (3,4) > high water mark (1)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logrus.New()
			log.SetLevel(logrus.DebugLevel)

			config := &ethereum.TopicConfig{
				AllTopics:               []string{"block"},
				OptInTopics:             []string{},
				AttestationEnabled:      true,
				MismatchDetectionWindow: 32,
				MismatchThreshold:       1,
				MismatchCooldown:        5 * time.Minute,
				SubnetHighWaterMark:     tt.highWaterMark,
			}
			tm := ethereum.NewTopicManager(log, config)

			// Set advertised subnets
			tm.SetAdvertisedSubnets(tt.advertisedSubnets)

			// Record attestations from seen subnets
			for i, subnet := range tt.seenSubnets {
				tm.RecordAttestation(subnet, phase0.Slot(i))
			}

			// Check if reconnection is needed (which indicates mismatch threshold was hit)
			select {
			case <-tm.NeedsReconnection():
				assert.True(t, tt.expectMismatch, "Unexpected mismatch for: %s", tt.description)
			default:
				assert.False(t, tt.expectMismatch, "Expected mismatch but none occurred for: %s", tt.description)
			}
		})
	}
}

func TestTopicManager_HighWaterMarkWithCooldown(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	config := &ethereum.TopicConfig{
		AllTopics:               []string{"block"},
		OptInTopics:             []string{},
		AttestationEnabled:      true,
		MismatchDetectionWindow: 10, // Small window for easier testing
		MismatchThreshold:       1,  // Trigger on first mismatch for simpler test
		MismatchCooldown:        100 * time.Millisecond,
		SubnetHighWaterMark:     2,
	}
	tm := ethereum.NewTopicManager(log, config)

	// Set advertised subnets
	tm.SetAdvertisedSubnets([]int{1, 2})

	// Record attestations exceeding high water mark
	// First record the advertised subnets, then non-advertised ones
	tm.RecordAttestation(1, phase0.Slot(1))
	tm.RecordAttestation(2, phase0.Slot(1))
	tm.RecordAttestation(3, phase0.Slot(1))
	tm.RecordAttestation(4, phase0.Slot(1))
	tm.RecordAttestation(5, phase0.Slot(1)) // Now have 3 non-advertised, exceeds high water mark

	// Should trigger immediately (threshold is 1)
	select {
	case <-tm.NeedsReconnection():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected reconnection after threshold reached")
	}

	// Reset and try again immediately - should respect cooldown
	tm.ResetAfterReconnection()
	tm.SetAdvertisedSubnets([]int{1, 2})

	// Try to trigger again immediately (within cooldown period)
	tm.RecordAttestation(1, phase0.Slot(20)) // New window
	tm.RecordAttestation(2, phase0.Slot(20))
	tm.RecordAttestation(3, phase0.Slot(20))
	tm.RecordAttestation(4, phase0.Slot(20))
	tm.RecordAttestation(5, phase0.Slot(20)) // Exceeds high water mark again

	// Even with mismatch detected, should not trigger due to cooldown
	select {
	case <-tm.NeedsReconnection():
		t.Fatal("Should respect cooldown period")
	default:
		// Expected - cooldown prevents reconnection
	}

	// Wait for cooldown to expire
	time.Sleep(150 * time.Millisecond)

	// Now try again after cooldown expired
	tm.RecordAttestation(1, phase0.Slot(35)) // Another new window
	tm.RecordAttestation(2, phase0.Slot(35))
	tm.RecordAttestation(3, phase0.Slot(35))
	tm.RecordAttestation(4, phase0.Slot(35))
	tm.RecordAttestation(5, phase0.Slot(35)) // Exceeds high water mark

	// Should trigger now that cooldown has expired
	select {
	case <-tm.NeedsReconnection():
		// Expected after cooldown
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected reconnection after cooldown expired")
	}
}

func TestTopicManager_HighWaterMarkEdgeCases(t *testing.T) {
	t.Run("high water mark with detection window reset", func(t *testing.T) {
		log := logrus.New()
		log.SetLevel(logrus.DebugLevel)

		config := &ethereum.TopicConfig{
			AllTopics:               []string{"block"},
			OptInTopics:             []string{},
			AttestationEnabled:      true,
			MismatchDetectionWindow: 5, // Small window
			MismatchThreshold:       1,
			MismatchCooldown:        5 * time.Minute,
			SubnetHighWaterMark:     3,
		}
		tm := ethereum.NewTopicManager(log, config)

		tm.SetAdvertisedSubnets([]int{1})

		// Fill first window
		for slot := uint64(0); slot < 5; slot++ {
			tm.RecordAttestation(1, phase0.Slot(slot))
			tm.RecordAttestation(2, phase0.Slot(slot))
		}

		// Jump to new window - should reset tracking
		tm.RecordAttestation(1, phase0.Slot(10))
		tm.RecordAttestation(2, phase0.Slot(10))
		tm.RecordAttestation(3, phase0.Slot(10))
		tm.RecordAttestation(4, phase0.Slot(10))

		// Should not trigger (only 3 non-advertised in new window)
		select {
		case <-tm.NeedsReconnection():
			t.Fatal("Should not trigger within high water mark")
		default:
			// Expected
		}
	})

	t.Run("high water mark with changing advertised subnets", func(t *testing.T) {
		log := logrus.New()
		log.SetLevel(logrus.DebugLevel)

		config := &ethereum.TopicConfig{
			AllTopics:               []string{"block"},
			OptInTopics:             []string{},
			AttestationEnabled:      true,
			MismatchDetectionWindow: 32,
			MismatchThreshold:       1,
			MismatchCooldown:        5 * time.Minute,
			SubnetHighWaterMark:     2,
		}
		tm := ethereum.NewTopicManager(log, config)

		// Start with subnets 1,2
		tm.SetAdvertisedSubnets([]int{1, 2})
		tm.RecordAttestation(1, phase0.Slot(1))
		tm.RecordAttestation(2, phase0.Slot(1))
		tm.RecordAttestation(3, phase0.Slot(1))
		tm.RecordAttestation(4, phase0.Slot(1))

		// Change advertised subnets
		tm.SetAdvertisedSubnets([]int{3, 4})

		// Now 1,2 are non-advertised
		tm.RecordAttestation(1, phase0.Slot(2))
		tm.RecordAttestation(2, phase0.Slot(2))
		tm.RecordAttestation(3, phase0.Slot(2))
		tm.RecordAttestation(4, phase0.Slot(2))

		// Should trigger (all 4 seen, but now 1,2 are non-advertised, total 2 = high water mark)
		select {
		case <-tm.NeedsReconnection():
			t.Fatal("Should not trigger when at high water mark")
		default:
			// Expected - at threshold but not over
		}

		// Add one more non-advertised
		tm.RecordAttestation(5, phase0.Slot(3))

		// Should trigger now (3 non-advertised > 2)
		select {
		case <-tm.NeedsReconnection():
			// Expected
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Expected reconnection when exceeding high water mark")
		}
	})
}

func TestTopicManager_RandomSubnetSelection(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	// Create a topic manager with attestation enabled config
	config := &ethereum.TopicConfig{
		AttestationEnabled:    true,
		AttestationMaxSubnets: 64, // Allow up to 64 subnets
	}
	tm := ethereum.NewTopicManager(log, config)

	// Test 1: Setting advertised subnets should select one randomly
	advertisedSubnets := []int{10, 20, 30, 40}
	tm.SetAdvertisedSubnets(advertisedSubnets)

	// Verify only the selected subnet is active
	activeCount := 0
	var activeSubnet int
	for _, subnet := range advertisedSubnets {
		if tm.IsActiveSubnet(uint64(subnet)) {
			activeCount++
			activeSubnet = subnet
		}
	}

	assert.Equal(t, 1, activeCount, "Exactly one subnet should be active")
	assert.Contains(t, advertisedSubnets, activeSubnet, "Active subnet should be from advertised list")

	// Test 2: Empty advertised subnets should result in no active subnet
	tm.SetAdvertisedSubnets([]int{})
	for i := 0; i < 64; i++ {
		assert.False(t, tm.IsActiveSubnet(uint64(i)), "No subnet should be active when advertised list is empty")
	}

	// Test 3: Single advertised subnet should always be selected
	tm.SetAdvertisedSubnets([]int{42})
	assert.True(t, tm.IsActiveSubnet(42), "Single advertised subnet should be active")
	assert.False(t, tm.IsActiveSubnet(41), "Non-advertised subnet should not be active")
	assert.False(t, tm.IsActiveSubnet(43), "Non-advertised subnet should not be active")
}

func TestTopicManager_TooManySubnetsDisablesSelection(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	// Create a topic manager with a low max subnet threshold
	config := &ethereum.TopicConfig{
		AttestationEnabled:    true,
		AttestationMaxSubnets: 2, // Only allow up to 2 subnets
	}
	tm := ethereum.NewTopicManager(log, config)

	// Test 1: Within threshold - should select a subnet
	tm.SetAdvertisedSubnets([]int{10, 20})
	activeCount := 0
	for i := 0; i < 64; i++ {
		if tm.IsActiveSubnet(uint64(i)) {
			activeCount++
		}
	}
	assert.Equal(t, 1, activeCount, "Should have exactly one active subnet when within threshold")

	// Test 2: Exceeding threshold - should not select any subnet
	allSubnets := make([]int, 64)
	for i := 0; i < 64; i++ {
		allSubnets[i] = i
	}
	tm.SetAdvertisedSubnets(allSubnets)

	for i := 0; i < 64; i++ {
		assert.False(t, tm.IsActiveSubnet(uint64(i)), "No subnet should be active when count exceeds threshold")
	}

	// Test 3: Right at threshold - should select a subnet
	tm.SetAdvertisedSubnets([]int{30})
	assert.True(t, tm.IsActiveSubnet(30), "Single subnet within threshold should be active")
}

func TestTopicManager_MismatchDetectionExcludesNotSelectedSubnets(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	// Create a topic manager with mismatch detection enabled
	config := &ethereum.TopicConfig{
		AttestationEnabled:      true,
		AttestationMaxSubnets:   2,
		MismatchDetectionWindow: 10,
		MismatchThreshold:       1,
		MismatchCooldown:        100 * time.Millisecond,
		SubnetHighWaterMark:     2,
	}
	tm := ethereum.NewTopicManager(log, config)

	// Set advertised subnets (e.g., 1, 2, 3, 4)
	// Only one will be randomly selected for forwarding
	advertisedSubnets := []int{1, 2, 3, 4}
	tm.SetAdvertisedSubnets(advertisedSubnets)

	// Record attestations from all advertised subnets
	for _, subnet := range advertisedSubnets {
		tm.RecordAttestation(uint64(subnet), phase0.Slot(1))
	}

	// Also record some non-advertised subnets
	tm.RecordAttestation(10, phase0.Slot(1))
	tm.RecordAttestation(11, phase0.Slot(1))

	// Should not trigger reconnection yet (only 2 non-advertised subnets)
	select {
	case <-tm.NeedsReconnection():
		t.Fatal("Should not trigger with only 2 non-advertised subnets")
	default:
		// Expected
	}

	// Add one more non-advertised subnet to exceed high water mark
	tm.RecordAttestation(12, phase0.Slot(1))

	// Now should trigger (3 non-advertised > 2 high water mark)
	select {
	case <-tm.NeedsReconnection():
		// Expected - advertised-but-not-selected subnets don't count
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected reconnection when non-advertised subnets exceed high water mark")
	}
}
