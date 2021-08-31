package txbuildtest

import (
	"fmt"

	"github.com/stellar/go/xdr"
)

// BuildResult returns a result XDR base64 encoded that is successful or not based on the input parameter.
func BuildResult(success bool) (string, error) {
	var code xdr.TransactionResultCode
	if success {
		code = xdr.TransactionResultCodeTxSuccess
	} else {
		code = xdr.TransactionResultCodeTxFailed
	}
	tr := xdr.TransactionResult{
		FeeCharged: 123,
		Result: xdr.TransactionResultResult{
			Code:    code,
			Results: &[]xdr.OperationResult{},
		},
	}
	trXDR, err := xdr.MarshalBase64(tr)
	if err != nil {
		return "", fmt.Errorf("converting transaction result to base64 xdr: %w", err)
	}
	return trXDR, nil
}

// BuildResultMeta returns a result meta XDR base64 encoded that contains arbitrary operations that result in the given account and trustline state.
func BuildResultMeta(ledgerEntryChanges xdr.LedgerEntryChanges) (string, error) {
	tm := xdr.TransactionMeta{
		V: 2,
		V2: &xdr.TransactionMetaV2{
			Operations: []xdr.OperationMeta{
				xdr.OperationMeta{
					Changes: ledgerEntryChanges,
				},
			},
		},
	}

	tmXDR, err := xdr.MarshalBase64(tm)
	if err != nil {
		return "", fmt.Errorf("converting transaction meta to base64 xdr: %w", err)
	}
	return tmXDR, nil
}
