package events

import (
	"time"

	"github.com/jellydator/ttlcache/v3"
)

type DuplicateCache struct {
	BeaconETHV1EventsBlock               *ttlcache.Cache[string, time.Time]
	BeaconETHV1EventsBlockGossip         *ttlcache.Cache[string, time.Time]
	BeaconETHV1EventsChainReorg          *ttlcache.Cache[string, time.Time]
	BeaconETHV1EventsFinalizedCheckpoint *ttlcache.Cache[string, time.Time]
	BeaconETHV1EventsHead                *ttlcache.Cache[string, time.Time]
	BeaconETHV1EventsBlobSidecar         *ttlcache.Cache[string, time.Time]
	BeaconETHV1EventsAttestationV2       *ttlcache.Cache[string, time.Time]
	BeaconETHV1EventsDataColumnSidecar   *ttlcache.Cache[string, time.Time]

	// Gloas (EIP-7732) topics.
	BeaconETHV1EventsHeadV2                    *ttlcache.Cache[string, time.Time]
	BeaconETHV1EventsExecutionPayload          *ttlcache.Cache[string, time.Time]
	BeaconETHV1EventsExecutionPayloadGossip    *ttlcache.Cache[string, time.Time]
	BeaconETHV1EventsExecutionPayloadAvailable *ttlcache.Cache[string, time.Time]
	BeaconETHV1EventsExecutionPayloadBid       *ttlcache.Cache[string, time.Time]
	BeaconETHV1EventsPayloadAttestation        *ttlcache.Cache[string, time.Time]
	BeaconETHV1EventsProposerPreferences       *ttlcache.Cache[string, time.Time]
}

const (
	// best to keep this > 1 epoch as some clients may send the same attestation on new epoch.
	TTL = 7 * time.Minute
)

func NewDuplicateCache() *DuplicateCache {
	return &DuplicateCache{
		BeaconETHV1EventsBlock: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
		BeaconETHV1EventsBlockGossip: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
		BeaconETHV1EventsChainReorg: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
		BeaconETHV1EventsFinalizedCheckpoint: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
		BeaconETHV1EventsHead: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
		BeaconETHV1EventsBlobSidecar: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
		BeaconETHV1EventsAttestationV2: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
		BeaconETHV1EventsDataColumnSidecar: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
		BeaconETHV1EventsHeadV2: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
		BeaconETHV1EventsExecutionPayload: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
		BeaconETHV1EventsExecutionPayloadGossip: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
		BeaconETHV1EventsExecutionPayloadAvailable: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
		BeaconETHV1EventsExecutionPayloadBid: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
		BeaconETHV1EventsPayloadAttestation: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
		BeaconETHV1EventsProposerPreferences: ttlcache.New(
			ttlcache.WithTTL[string, time.Time](TTL),
		),
	}
}

func (d *DuplicateCache) Start() {
	go d.BeaconETHV1EventsBlock.Start()
	go d.BeaconETHV1EventsBlockGossip.Start()
	go d.BeaconETHV1EventsChainReorg.Start()
	go d.BeaconETHV1EventsFinalizedCheckpoint.Start()
	go d.BeaconETHV1EventsHead.Start()
	go d.BeaconETHV1EventsBlobSidecar.Start()
	go d.BeaconETHV1EventsAttestationV2.Start()
	go d.BeaconETHV1EventsDataColumnSidecar.Start()
	go d.BeaconETHV1EventsHeadV2.Start()
	go d.BeaconETHV1EventsExecutionPayload.Start()
	go d.BeaconETHV1EventsExecutionPayloadGossip.Start()
	go d.BeaconETHV1EventsExecutionPayloadAvailable.Start()
	go d.BeaconETHV1EventsExecutionPayloadBid.Start()
	go d.BeaconETHV1EventsPayloadAttestation.Start()
	go d.BeaconETHV1EventsProposerPreferences.Start()
}
