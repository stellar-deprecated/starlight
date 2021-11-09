package horizon

import (
	"fmt"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/starlight/sdk/agent/submit"
)

var _ submit.SubmitTxer = &Submitter{}

// Submitter implements an submit's interface for submitting transaction XDRs to
// the network, via Horizon's API.
type Submitter struct {
	HorizonClient horizonclient.ClientInterface
}

// SubmitTx submits the given xdr as a transaction to Horizon.
func (h *Submitter) SubmitTx(xdr string) error {
	_, err := h.HorizonClient.SubmitTransactionXDR(xdr)
	if err != nil {
		return fmt.Errorf("submitting tx %s: %w", xdr, buildErr(err))
	}
	return nil
}

func buildErr(err error) error {
	if hErr := horizonclient.GetError(err); hErr != nil {
		resultString, rErr := hErr.ResultString()
		if rErr != nil {
			resultString = "<error getting result string: " + rErr.Error() + ">"
		}
		return fmt.Errorf("%w (%v)", err, resultString)
	}
	return err
}
