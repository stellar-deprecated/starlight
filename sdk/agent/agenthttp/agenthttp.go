// Package agenthttp contains a simple HTTP handler that, when requested, will
// return a snapshot of an agent's snapshot at that moment.
package agenthttp

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/cors"
	"github.com/stellar/go/keypair"
	"github.com/stellar/starlight/sdk/agent"
)

// New creates a new http.Handler that returns snapshots for the given agent at
// the root path. The snapshot generated is also accompanied by the agents
// config that was used to create the agent at the time it was created. Secrets
// keys in the config are transformed into public keys.
func New(a *agent.Agent) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/", handleSnapshot(a))
	return cors.Default().Handler(m)
}

func handleSnapshot(a *agent.Agent) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		type agentConfig struct {
			ObservationPeriodTime      time.Duration
			ObservationPeriodLedgerGap uint32
			MaxOpenExpiry              time.Duration
			NetworkPassphrase          string
			ChannelAccountKey          *keypair.FromAddress
			ChannelAccountSigner       *keypair.FromAddress
		}
		c := a.Config()
		v := struct {
			Config   agentConfig
			Snapshot agent.Snapshot
		}{
			Config: agentConfig{
				ObservationPeriodTime:      c.ObservationPeriodTime,
				ObservationPeriodLedgerGap: c.ObservationPeriodLedgerGap,
				MaxOpenExpiry:              c.MaxOpenExpiry,
				NetworkPassphrase:          c.NetworkPassphrase,
				ChannelAccountKey:          c.ChannelAccountKey,
				ChannelAccountSigner:       c.ChannelAccountSigner.FromAddress(),
			},
			Snapshot: a.Snapshot(),
		}
		err := enc.Encode(v)
		if err != nil {
			panic(err)
		}
	}
}
