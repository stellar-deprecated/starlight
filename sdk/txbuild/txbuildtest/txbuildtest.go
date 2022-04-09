package txbuildtest

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

// BuildResult returns a result XDR base64 encoded that is successful or not
// based on the input parameter.
func BuildResultXDR(success bool) (string, error) {
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
		return "", fmt.Errorf("encoding transaction result to base64 xdr: %w", err)
	}
	return trXDR, nil
}

// BuildResultMetaXDR returns a result meta XDR base64 encoded that contains
// the input ledger entry changes. Only creates one operation meta for
// simiplicity.
func BuildResultMetaXDR(ledgerEntryResults []xdr.LedgerEntryData) (string, error) {
	tm := xdr.TransactionMeta{
		V: 2,
		V2: &xdr.TransactionMetaV2{
			Operations: []xdr.OperationMeta{
				{},
			},
		},
	}

	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	for _, result := range ledgerEntryResults {
		change := xdr.LedgerEntryChange{}
		// When operations like ChangeTrustOp execute they potentially create or
		// update existing ledger entries. The operation encoded in the
		// transaction doesn't actually know which will occur because it depends
		// on the state of the network at the time the transaction is executed.
		// Randomly simulating whether a create or update change occurs
		// simulates that uncertainty while also ensuring we broadly test that
		// different behavior across all of our tests.
		if rand.Int()%2 == 0 {
			change.Type = xdr.LedgerEntryChangeTypeLedgerEntryCreated
			change.Created = &xdr.LedgerEntry{Data: result}
		} else {
			change.Type = xdr.LedgerEntryChangeTypeLedgerEntryUpdated
			change.Updated = &xdr.LedgerEntry{Data: result}
		}
		tm.V2.Operations[0].Changes = append(tm.V2.Operations[0].Changes, change)
	}

	tmXDR, err := xdr.MarshalBase64(tm)
	if err != nil {
		return "", fmt.Errorf("encoding transaction meta to base64 xdr: %w", err)
	}
	return tmXDR, nil
}

type OpenResultMetaParams struct {
	InitiatorSigner         string
	InitiatorSignerWeight   uint32 // Defaults to 1 if not set.
	ResponderSigner         string
	ResponderSignerWeight   uint32 // Defaults to 1 if not set.
	ExtraSigner             string
	InitiatorChannelAccount string
	ResponderChannelAccount string
	Thresholds              xdr.Thresholds // Defaults to 0, 2, 2, 2 if not set.
	StartSequence           int64
	Asset                   txnbuild.Asset
	TrustLineFlag               xdr.TrustLineFlags // Defaults to authorized flag.
}

func BuildOpenResultMetaXDR(params OpenResultMetaParams) (string, error) {
	initiatorSignerWeight := uint32(1)
	if params.InitiatorSignerWeight > 0 {
		initiatorSignerWeight = params.InitiatorSignerWeight
	}
	responderSignerWeight := uint32(1)
	if params.ResponderSignerWeight > 0 {
		responderSignerWeight = params.ResponderSignerWeight
	}
	thresholds := xdr.Thresholds{0, 2, 2, 2}
	if params.Thresholds != (xdr.Thresholds{}) {
		thresholds = params.Thresholds
	}
	led := []xdr.LedgerEntryData{
		{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: xdr.MustAddress(params.InitiatorChannelAccount),
				SeqNum:    xdr.SequenceNumber(params.StartSequence),
				Signers: []xdr.Signer{
					{
						Key:    xdr.MustSigner(params.InitiatorSigner),
						Weight: xdr.Uint32(initiatorSignerWeight),
					},
					{
						Key:    xdr.MustSigner(params.ResponderSigner),
						Weight: xdr.Uint32(responderSignerWeight),
					},
				},
				Thresholds: thresholds,
			},
		},
		{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: xdr.MustAddress(params.ResponderChannelAccount),
				SeqNum:    xdr.SequenceNumber(1),
				Signers: []xdr.Signer{
					{
						Key:    xdr.MustSigner(params.InitiatorSigner),
						Weight: xdr.Uint32(initiatorSignerWeight),
					},
					{
						Key:    xdr.MustSigner(params.ResponderSigner),
						Weight: xdr.Uint32(responderSignerWeight),
					},
				},
				Thresholds: thresholds,
			},
		},
	}
	if params.ExtraSigner != "" {
		led[0].Account.Signers = append(led[0].Account.Signers, xdr.Signer{
			Key:    xdr.MustSigner(params.ExtraSigner),
			Weight: 1,
		})
		led[1].Account.Signers = append(led[0].Account.Signers, xdr.Signer{
			Key:    xdr.MustSigner(params.ExtraSigner),
			Weight: 1,
		})
	}

	if !params.Asset.IsNative() {
		trustlineFlag := xdr.TrustLineFlagsAuthorizedFlag
		if params.TrustLineFlag != 0 {
			trustlineFlag = params.TrustLineFlag
		}
		led = append(led, []xdr.LedgerEntryData{
			{
				Type: xdr.LedgerEntryTypeTrustline,
				TrustLine: &xdr.TrustLineEntry{
					AccountId: xdr.MustAddress(params.InitiatorChannelAccount),
					Balance:   0,
					Asset:     xdr.MustNewCreditAsset(params.Asset.GetCode(), params.Asset.GetIssuer()).ToTrustLineAsset(),
					Flags:     xdr.Uint32(trustlineFlag),
				},
			},
			{
				Type: xdr.LedgerEntryTypeTrustline,
				TrustLine: &xdr.TrustLineEntry{
					AccountId: xdr.MustAddress(params.ResponderChannelAccount),
					Balance:   0,
					Asset:     xdr.MustNewCreditAsset(params.Asset.GetCode(), params.Asset.GetIssuer()).ToTrustLineAsset(),
					Flags:     xdr.Uint32(trustlineFlag),
				},
			},
		}...)
	}

	return BuildResultMetaXDR(led)
}
