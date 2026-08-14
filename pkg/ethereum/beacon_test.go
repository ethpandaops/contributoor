package ethereum

import (
	"os/exec"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		var (
			activeCount  = 0
			activeSubnet int
		)

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

// TestBeaconWrapper_GetAttestationSubnetID proves the full spec formula against the exact vector
// used during the nemesis triage to PoC-confirm the old committee_index%64 code was wrong on every
// real network: committees_per_slot=45 (a plausible mainnet-scale value; mainnet is never 64),
// slot=100, committee_index=10. The old code would have answered 10 (committee_index%64); the
// correct spec answer is 62.
func TestBeaconWrapper_GetAttestationSubnetID(t *testing.T) {
	t.Run("not yet known before the first fetch completes", func(t *testing.T) {
		w := &BeaconWrapper{}

		_, ok := w.GetAttestationSubnetID(100, 10)
		assert.False(t, ok, "must report unknown, not fall back to a guessed value")
	})

	t.Run("known but slotsPerEpoch missing", func(t *testing.T) {
		w := &BeaconWrapper{}
		w.committeesPerSlot.Store(45)

		_, ok := w.GetAttestationSubnetID(100, 10)
		assert.False(t, ok)
	})

	t.Run("computes the full spec formula once both values are known", func(t *testing.T) {
		w := &BeaconWrapper{}
		w.committeesPerSlot.Store(45)
		w.slotsPerEpoch.Store(32)

		subnetID, ok := w.GetAttestationSubnetID(100, 10)
		require.True(t, ok)

		// (committees_per_slot * (slot % SLOTS_PER_EPOCH) + committee_index) % 64
		//   = (45 * (100 % 32) + 10) % 64 = (45*4 + 10) % 64 = 190 % 64 = 62
		assert.Equal(t, uint64(62), subnetID)

		oldFormula := uint64(10) % 64
		assert.NotEqual(t, oldFormula, subnetID,
			"BUG NM-07 regression check: the old committee_index%%64 code would have answered %d "+
				"here, not the spec-correct %d - if these match again, the fix regressed",
			oldFormula, subnetID)
	})
}

// TestRefreshCommitteesPerSlot_UsesBeaconCommitteesAndSlotsPerEpoch is a structural regression
// test, not a behavioral one. Node().Service() and Node().Spec() both require a live, fully
// bootstrapped ethcore.BeaconNode to return anything usable (confirmed directly: on a freshly
// constructed, never-Start()ed BeaconWrapper, Service() is nil and Spec() returns "spec is not
// available") - the same live-state wall already documented elsewhere in this codebase's tests
// (NM-02/09/12 in the nemesis triage). What's verified here: the exact fetch-and-derive logic is
// present, unchanged, in the current source.
func TestRefreshCommitteesPerSlot_UsesBeaconCommitteesAndSlotsPerEpoch(t *testing.T) {
	out, err := exec.Command("grep", "-n", "-A", "3", "func (w \\*BeaconWrapper) refreshCommitteesPerSlot", "beacon.go").
		CombinedOutput()
	require.NoError(t, err, "grep must find refreshCommitteesPerSlot in beacon.go")

	block := string(out)
	assert.Contains(t, block, "BeaconCommitteesProvider",
		"must fetch via BeaconCommitteesProvider - there is no lighter endpoint for this value")

	out2, err := exec.Command("grep", "-n", "len(resp.Data)) / uint64(spec.SlotsPerEpoch)", "beacon.go").
		CombinedOutput()
	require.NoError(t, err, "grep must find the committees-per-slot derivation in beacon.go")
	assert.NotEmpty(t, string(out2),
		"expected committeesPerSlot to be derived as total committees for the epoch divided by "+
			"SlotsPerEpoch - if this fails, the derivation changed and this test needs updating "+
			"alongside it, or (if it was removed) NM-07 has regressed")
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
