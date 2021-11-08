package main

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/stellar/go/keypair"
	agentpkg "github.com/stellar/starlight/sdk/agent"
)

type File struct {
	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	MaxOpenExpiry              time.Duration
	EscrowAccountKey           *keypair.FromAddress
	Snapshot                   agentpkg.Snapshot
}

type JSONFileSnapshotter struct {
	Filename string

	ObservationPeriodTime      time.Duration
	ObservationPeriodLedgerGap int64
	MaxOpenExpiry              time.Duration
	EscrowAccountKey           *keypair.FromAddress
}

func (j JSONFileSnapshotter) Snapshot(a *agentpkg.Agent, s agentpkg.Snapshot) {
	f := File{
		ObservationPeriodTime:      j.ObservationPeriodTime,
		ObservationPeriodLedgerGap: j.ObservationPeriodLedgerGap,
		MaxOpenExpiry:              j.MaxOpenExpiry,
		EscrowAccountKey:           j.EscrowAccountKey,
		Snapshot:                   s,
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(j.Filename, b, 0644)
	if err != nil {
		panic(err)
	}
}
