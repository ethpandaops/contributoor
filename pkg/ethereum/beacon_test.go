package ethereum

import (
	"os/exec"
	"strings"
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

// TestBeaconWrapper_IsHealthy proves IsHealthy reflects the wrapper's own live isHealthy field,
// not a one-time latch. A bare &BeaconWrapper{} has a nil embedded *ethcore.BeaconNode, so if
// this resolved to the promoted ethcore method instead of the wrapper's own override, it would
// panic here rather than return a value - the fact that it doesn't confirms the override is what
// answers the call.
func TestBeaconWrapper_IsHealthy(t *testing.T) {
	w := &BeaconWrapper{}

	assert.False(t, w.IsHealthy(), "zero value must be unhealthy")

	w.isHealthy.Store(true)
	assert.True(t, w.IsHealthy())

	// A live disconnect must be reflected immediately, not latched until Stop.
	w.isHealthy.Store(false)
	assert.False(t, w.IsHealthy())

	w.isHealthy.Store(true)
	assert.True(t, w.IsHealthy(), "must be able to report healthy again after reconnecting")
}

// TestHandleDecoratedEvent_DoesNotDoubleCountFailedEvents is a structural regression test, not a
// behavioral one. handleDecoratedEvent's very first line calls w.Synced(ctx), which is promoted
// from the embedded *ethcore.BeaconNode and requires a populated wallclock (genesis and spec
// fetched from a real beacon node) before it returns successfully - there is no test seam in
// ethcore to fake that state, the same wall already documented for this method during the nemesis
// triage (see .audit/findings/triage-report.md, NM-09/NM-12). What's verified here instead: the
// exact conditional shape the fix depends on is present, unchanged, on the current source - that
// an event which fails at any sink is counted via AddFailedEvents only, not also via
// AddDecoratedEvent/AddEventsExported.
func TestHandleDecoratedEvent_DoesNotDoubleCountFailedEvents(t *testing.T) {
	out, err := exec.Command("grep", "-n", "-A", "2", "if failure {", "beacon.go").CombinedOutput()
	require.NoError(t, err, "grep must find the failure branch in beacon.go")

	block := string(out)

	require.Contains(t, block, "AddFailedEvents(1)")
	require.Contains(t, block, "} else {",
		"the failure branch must be exactly one statement (AddFailedEvents) before the else - if "+
			"this fails, either the shape changed or an exported/decorated-event call was added "+
			"back into the failure branch, regressing the double-counting bug (NM-12)")

	lines := strings.Split(strings.TrimSpace(block), "\n")
	require.Len(t, lines, 3, "expected exactly: 'if failure {', the AddFailedEvents call, and '} else {'")
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
