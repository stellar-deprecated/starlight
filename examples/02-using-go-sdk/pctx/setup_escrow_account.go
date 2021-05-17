package pctx

import (
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
)

func SetupEscrowAccount(networkPassphrase string, client horizonclient.ClientInterface, creator *keypair.Full, account *keypair.Full, initialContribution string) error {
	var err error

	sourceAccount, err := client.AccountDetail(horizonclient.AccountRequest{AccountID: creator.Address()})
	if err != nil {
		return err
	}
	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Timebounds:           txnbuild.NewTimeout(300),
			Operations: []txnbuild.Operation{
				&txnbuild.BeginSponsoringFutureReserves{
					SponsoredID: account.Address(),
				},
				&txnbuild.CreateAccount{
					Destination: account.Address(),
					Amount:      initialContribution,
				},
				&txnbuild.SetOptions{
					SourceAccount:   account.Address(),
					MasterWeight:    txnbuild.NewThreshold(0),
					LowThreshold:    txnbuild.NewThreshold(1),
					MediumThreshold: txnbuild.NewThreshold(1),
					HighThreshold:   txnbuild.NewThreshold(1),
					Signer:          &txnbuild.Signer{Address: creator.Address(), Weight: 1},
				},
				&txnbuild.EndSponsoringFutureReserves{
					SourceAccount: account.Address(),
				},
			},
		},
	)
	if err != nil {
		return err
	}
	tx, err = tx.Sign(networkPassphrase, creator, account)
	if err != nil {
		return err
	}
	_, err = client.SubmitTransaction(tx)
	if err != nil {
		return err
	}
	return nil
}
