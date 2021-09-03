package horizon

import (
	"fmt"

	"github.com/stellar/go/clients/horizonclient"
)

type Submitter struct {
	HorizonClient horizonclient.ClientInterface
}

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
