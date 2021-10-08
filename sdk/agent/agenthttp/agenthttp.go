package agenthttp

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/cors"
	"github.com/stellar/experimental-payment-channels/sdk/agent"
	"github.com/stellar/go/keypair"
)

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
			ObservationPeriodLedgerGap int64
			MaxOpenExpiry              time.Duration
			NetworkPassphrase          string
			EscrowAccountKey           *keypair.FromAddress
			EscrowAccountSigner        *keypair.FromAddress
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
				EscrowAccountKey:           c.EscrowAccountKey,
				EscrowAccountSigner:        c.EscrowAccountSigner.FromAddress(),
			},
			Snapshot: a.Snapshot(),
		}
		err := enc.Encode(v)
		if err != nil {
			panic(err)
		}
	}
}
