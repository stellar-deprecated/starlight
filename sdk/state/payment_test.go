package state

import (
	"strconv"
	"testing"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/txbuildtest"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloseAgreement_Equal(t *testing.T) {
	testCases := []struct {
		ca1       CloseAgreement
		ca2       CloseAgreement
		wantEqual bool
	}{
		{CloseAgreement{}, CloseAgreement{}, true},
		{
			CloseAgreement{
				Details: CloseAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					IterationNumber:            3,
					Balance:                    100,
				},
			},
			CloseAgreement{
				Details: CloseAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					IterationNumber:            3,
					Balance:                    100,
				},
			},
			true,
		},
		{
			CloseAgreement{
				Details: CloseAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					IterationNumber:            3,
					Balance:                    100,
				},
			},
			CloseAgreement{},
			false,
		},
		{
			CloseAgreement{
				Details: CloseAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					IterationNumber:            3,
					Balance:                    100,
				},
				ProposerSignatures: CloseAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			CloseAgreement{
				Details: CloseAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					IterationNumber:            3,
					Balance:                    100,
				},
				ProposerSignatures: CloseAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			true,
		},
		{
			CloseAgreement{
				Details: CloseAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					IterationNumber:            3,
					Balance:                    100,
				},
				ProposerSignatures: CloseAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			CloseAgreement{},
			false,
		},
		{
			CloseAgreement{
				Details: CloseAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					IterationNumber:            3,
					Balance:                    100,
				},
				ProposerSignatures: CloseAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			CloseAgreement{
				Details: CloseAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					IterationNumber:            3,
					Balance:                    100,
				},
				ProposerSignatures: CloseAgreementSignatures{
					Close: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			false,
		},
		{
			CloseAgreement{
				Details: CloseAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					IterationNumber:            3,
					Balance:                    100,
					ConfirmingSigner:           keypair.MustParseAddress("GCJFS4LZFAM7NKFQFEWE4W2SCGARSE2SMLGNWGHH2LSZ6CLX326MZWPO"),
				},
				ProposerSignatures: CloseAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			CloseAgreement{
				Details: CloseAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					IterationNumber:            3,
					Balance:                    100,
					ConfirmingSigner:           keypair.MustParseAddress("GCJFS4LZFAM7NKFQFEWE4W2SCGARSE2SMLGNWGHH2LSZ6CLX326MZWPO"),
				},
				ProposerSignatures: CloseAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			true,
		},
		{
			CloseAgreement{
				Details: CloseAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					IterationNumber:            3,
					Balance:                    100,
					ConfirmingSigner:           keypair.MustParseAddress("GCJFS4LZFAM7NKFQFEWE4W2SCGARSE2SMLGNWGHH2LSZ6CLX326MZWPO"),
				},
				ProposerSignatures: CloseAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			CloseAgreement{
				Details: CloseAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					IterationNumber:            3,
					Balance:                    100,
					ConfirmingSigner:           keypair.MustParseAddress("GDJ5SXSKKFXINP7TN4J4T4JAXL4VZL7UMIAGZWQTYSKHSNHLSPVOAXRY"),
				},
				ProposerSignatures: CloseAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			false,
		},
		{
			CloseAgreement{
				Details: CloseAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					IterationNumber:            3,
					Balance:                    100,
					ConfirmingSigner:           keypair.MustParseAddress("GCJFS4LZFAM7NKFQFEWE4W2SCGARSE2SMLGNWGHH2LSZ6CLX326MZWPO"),
				},
				ProposerSignatures: CloseAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			CloseAgreement{
				Details: CloseAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					IterationNumber:            3,
					Balance:                    100,
					ConfirmingSigner:           nil,
				},
				ProposerSignatures: CloseAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			false,
		},
	}
	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			equal := tc.ca1.Equal(tc.ca2)
			assert.Equal(t, tc.wantEqual, equal)
		})
	}
}

func TestChannel_ConfirmPayment_acceptsSameObservationPeriod(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildFormationResultMetaXDR(txbuildtest.FormationResultMetaParams{
			InitiatorSigner: localSigner.Address(),
			ResponderSigner: remoteSigner.Address(),
			InitiatorEscrow: localEscrowAccount.Address(),
			ResponderEscrow: remoteEscrowAccount.Address(),
			StartSequence:   101,
			Asset:           txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// A close agreement from the remote participant should be accepted if the
	// observation period matches the channels observation period.
	{
		initiatorChannel.latestAuthorizedCloseAgreement = CloseAgreement{
			Details: CloseAgreementDetails{
				ObservationPeriodTime:      1,
				ObservationPeriodLedgerGap: 1,
				ConfirmingSigner:           localSigner.FromAddress(),
			},
		}

		txs, err := initiatorChannel.closeTxs(initiatorChannel.openAgreement.Details, CloseAgreementDetails{
			IterationNumber:            1,
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 1,
			ProposingSigner:            remoteSigner.FromAddress(),
			ConfirmingSigner:           localSigner.FromAddress(),
		})
		txDecl := txs.Declaration
		txClose := txs.Close
		require.NoError(t, err)
		txDecl, err = txDecl.Sign(network.TestNetworkPassphrase, remoteSigner)
		require.NoError(t, err)
		txClose, err = txClose.Sign(network.TestNetworkPassphrase, remoteSigner)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmPayment(CloseAgreement{
			Details: CloseAgreementDetails{
				IterationNumber:            1,
				ObservationPeriodTime:      1,
				ObservationPeriodLedgerGap: 1,
				ProposingSigner:            remoteSigner.FromAddress(),
				ConfirmingSigner:           localSigner.FromAddress(),
			},
			ProposerSignatures: CloseAgreementSignatures{
				Declaration: txDecl.Signatures()[0].Signature,
				Close:       txClose.Signatures()[0].Signature,
			},
		})
		require.NoError(t, err)
	}
}

func TestChannel_ConfirmPayment_rejectsDifferentObservationPeriod(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildFormationResultMetaXDR(txbuildtest.FormationResultMetaParams{
			InitiatorSigner: localSigner.Address(),
			ResponderSigner: remoteSigner.Address(),
			InitiatorEscrow: localEscrowAccount.Address(),
			ResponderEscrow: remoteEscrowAccount.Address(),
			StartSequence:   101,
			Asset:           txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	initiatorChannel.latestAuthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 1,
			ConfirmingSigner:           localSigner.FromAddress(),
		},
	}

	// A close agreement from the remote participant should be rejected if the
	// observation period doesn't match the channels observation period.
	{
		txs, err := initiatorChannel.closeTxs(initiatorChannel.openAgreement.Details, CloseAgreementDetails{
			IterationNumber:            1,
			ObservationPeriodTime:      0,
			ObservationPeriodLedgerGap: 0,
			ConfirmingSigner:           localSigner.FromAddress(),
		})
		txDecl := txs.Declaration
		txClose := txs.Close
		require.NoError(t, err)
		txDecl, err = txDecl.Sign(network.TestNetworkPassphrase, remoteSigner)
		require.NoError(t, err)
		txClose, err = txClose.Sign(network.TestNetworkPassphrase, remoteSigner)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmPayment(CloseAgreement{
			Details: CloseAgreementDetails{
				IterationNumber:            1,
				ObservationPeriodTime:      0,
				ObservationPeriodLedgerGap: 0,
			},
			ProposerSignatures: CloseAgreementSignatures{
				Declaration: txDecl.Signatures()[0].Signature,
				Close:       txClose.Signatures()[0].Signature,
			},
		})
		require.EqualError(t, err, "validating payment: invalid payment observation period: different than channel state")
	}
}

func TestChannel_ConfirmPayment_localWhoIsInitiatorRejectsPaymentToRemoteWhoIsResponder(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildFormationResultMetaXDR(txbuildtest.FormationResultMetaParams{
			InitiatorSigner: localSigner.Address(),
			ResponderSigner: remoteSigner.Address(),
			InitiatorEscrow: localEscrowAccount.Address(),
			ResponderEscrow: remoteEscrowAccount.Address(),
			StartSequence:   101,
			Asset:           txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// A close agreement from the remote participant should be rejected if the
	// payment changes the balance in the favor of the remote.
	initiatorChannel.latestAuthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			IterationNumber:            1,
			Balance:                    100, // Local (initiator) owes remote (responder) 100.
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
			ConfirmingSigner:           localSigner.FromAddress(),
		},
	}

	ca := CloseAgreementDetails{
		IterationNumber:            2,
		Balance:                    110, // Local (initiator) owes remote (responder) 110, payment of 10 from ❌ local to remote.
		PaymentAmount:              -10, // Not possible to have a negative payment amount, but hardcode to test this validation.
		ProposingSigner:            remoteSigner.FromAddress(),
		ConfirmingSigner:           localSigner.FromAddress(),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
	}
	txs, err := initiatorChannel.closeTxs(initiatorChannel.openAgreement.Details, ca)
	txDecl := txs.Declaration
	txClose := txs.Close
	require.NoError(t, err)
	txDecl, err = txDecl.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	txClose, err = txClose.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmPayment(CloseAgreement{
		Details: ca,
		ProposerSignatures: CloseAgreementSignatures{
			Declaration: txDecl.Signatures()[0].Signature,
			Close:       txClose.Signatures()[0].Signature,
		},
	})
	require.EqualError(t, err, "close agreement is a payment to the proposer")
}

func TestChannel_ConfirmPayment_localWhoIsResponderRejectsPaymentToRemoteWhoIsInitiator(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildFormationResultMetaXDR(txbuildtest.FormationResultMetaParams{
			InitiatorSigner: remoteSigner.Address(),
			ResponderSigner: localSigner.Address(),
			InitiatorEscrow: remoteEscrowAccount.Address(),
			ResponderEscrow: localEscrowAccount.Address(),
			StartSequence:   101,
			Asset:           txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = responderChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// A close agreement from the remote participant should be rejected if the
	// payment changes the balance in the favor of the remote.
	responderChannel.latestAuthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			IterationNumber:            1,
			Balance:                    100, // Remote (initiator) owes local (responder) 100.
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
			ConfirmingSigner:           localSigner.FromAddress(),
		},
	}
	ca := CloseAgreementDetails{
		IterationNumber:            2,
		Balance:                    90,  // Remote (initiator) owes local (responder) 90, payment of 10 from ❌ local to remote.
		PaymentAmount:              -10, // Not possible to have a negative payment amount, but hardcode to test this validation.
		ProposingSigner:            remoteSigner.FromAddress(),
		ConfirmingSigner:           localSigner.FromAddress(),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
	}

	txs, err := responderChannel.closeTxs(responderChannel.openAgreement.Details, ca)
	txDecl := txs.Declaration
	txClose := txs.Close
	require.NoError(t, err)
	txDecl, err = txDecl.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	txClose, err = txClose.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	_, err = responderChannel.ConfirmPayment(CloseAgreement{
		Details: ca,
		ProposerSignatures: CloseAgreementSignatures{
			Declaration: txDecl.Signatures()[0].Signature,
			Close:       txClose.Signatures()[0].Signature,
		},
	})
	require.EqualError(t, err, "close agreement is a payment to the proposer")
}

func TestChannel_ConfirmPayment_initiatorRejectsPaymentThatIsUnderfunded(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildFormationResultMetaXDR(txbuildtest.FormationResultMetaParams{
			InitiatorSigner: localSigner.Address(),
			ResponderSigner: remoteSigner.Address(),
			InitiatorEscrow: localEscrowAccount.Address(),
			ResponderEscrow: remoteEscrowAccount.Address(),
			StartSequence:   101,
			Asset:           txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// A close agreement from the remote participant should be rejected if the
	// payment changes the balance in the favor of the remote.
	initiatorChannel.latestAuthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			IterationNumber:            1,
			Balance:                    -60, // Remote (responder) owes local (initiator) 60.
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
			ConfirmingSigner:           localSigner.FromAddress(),
		},
	}

	ca := CloseAgreementDetails{
		IterationNumber:            2,
		Balance:                    -110, // Remote (responder) owes local (initiator) 110, which responder ❌ cannot pay.
		PaymentAmount:              50,
		ProposingSigner:            remoteSigner.FromAddress(),
		ConfirmingSigner:           localSigner.FromAddress(),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
	}
	txs, err := initiatorChannel.closeTxs(initiatorChannel.openAgreement.Details, ca)
	txDecl := txs.Declaration
	txClose := txs.Close
	require.NoError(t, err)
	txDecl, err = txDecl.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	txClose, err = txClose.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmPayment(CloseAgreement{
		Details: ca,
		ProposerSignatures: CloseAgreementSignatures{
			Declaration: txDecl.Signatures()[0].Signature,
			Close:       txClose.Signatures()[0].Signature,
		},
	})
	assert.EqualError(t, err, "close agreement over commits: account is underfunded to make payment")
	assert.ErrorIs(t, err, ErrUnderfunded)

	// The same close payment should pass if the balance has been updated.
	initiatorChannel.UpdateRemoteEscrowAccountBalance(200)
	_, err = initiatorChannel.ConfirmPayment(CloseAgreement{
		Details: ca,
		ProposerSignatures: CloseAgreementSignatures{
			Declaration: txDecl.Signatures()[0].Signature,
			Close:       txClose.Signatures()[0].Signature,
		},
	})
	assert.NoError(t, err)
}

func TestChannel_ConfirmPayment_responderRejectsPaymentThatIsUnderfunded(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildFormationResultMetaXDR(txbuildtest.FormationResultMetaParams{
			InitiatorSigner: remoteSigner.Address(),
			ResponderSigner: localSigner.Address(),
			InitiatorEscrow: remoteEscrowAccount.Address(),
			ResponderEscrow: localEscrowAccount.Address(),
			StartSequence:   101,
			Asset:           txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = responderChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// A close agreement from the remote participant should be rejected if the
	// payment changes the balance in the favor of the remote.
	responderChannel.latestAuthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			IterationNumber:            1,
			Balance:                    60, // Remote (initiator) owes local (responder) 60.
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
			ConfirmingSigner:           localSigner.FromAddress(),
		},
	}

	ca := CloseAgreementDetails{
		IterationNumber:            2,
		Balance:                    110, // Remote (initiator) owes local (responder) 110, which initiator ❌ cannot pay.
		PaymentAmount:              50,
		ProposingSigner:            remoteSigner.FromAddress(),
		ConfirmingSigner:           localSigner.FromAddress(),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
	}
	txs, err := responderChannel.closeTxs(responderChannel.openAgreement.Details, ca)
	txDecl := txs.Declaration
	txClose := txs.Close
	require.NoError(t, err)
	txDecl, err = txDecl.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	txClose, err = txClose.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	_, err = responderChannel.ConfirmPayment(CloseAgreement{
		Details: ca,
		ProposerSignatures: CloseAgreementSignatures{
			Declaration: txDecl.Signatures()[0].Signature,
			Close:       txClose.Signatures()[0].Signature,
		},
	})
	assert.EqualError(t, err, "close agreement over commits: account is underfunded to make payment")
	assert.ErrorIs(t, err, ErrUnderfunded)

	// The same close payment should pass if the balance has been updated.
	responderChannel.UpdateRemoteEscrowAccountBalance(200)
	_, err = responderChannel.ConfirmPayment(CloseAgreement{
		Details: ca,
		ProposerSignatures: CloseAgreementSignatures{
			Declaration: txDecl.Signatures()[0].Signature,
			Close:       txClose.Signatures()[0].Signature,
		},
	})
	assert.NoError(t, err)
}

func TestChannel_ConfirmPayment_initiatorCannotProposePaymentThatIsUnderfunded(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildFormationResultMetaXDR(txbuildtest.FormationResultMetaParams{
			InitiatorSigner: localSigner.Address(),
			ResponderSigner: remoteSigner.Address(),
			InitiatorEscrow: localEscrowAccount.Address(),
			ResponderEscrow: remoteEscrowAccount.Address(),
			StartSequence:   101,
			Asset:           txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// A close agreement from the remote participant should be rejected if the
	// payment changes the balance in the favor of the remote.
	initiatorChannel.latestAuthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			IterationNumber:            1,
			Balance:                    60, // Local (initiator) owes remote (responder) 60.
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
			ConfirmingSigner:           localSigner.FromAddress(),
		},
	}

	_, err := initiatorChannel.ProposePayment(110)
	assert.EqualError(t, err, "amount over commits: account is underfunded to make payment")
	assert.ErrorIs(t, err, ErrUnderfunded)

	// The same close payment should pass if the balance has been updated.
	initiatorChannel.UpdateLocalEscrowAccountBalance(200)
	_, err = initiatorChannel.ProposePayment(110)
	assert.NoError(t, err)
}

func TestChannel_ConfirmPayment_responderCannotProposePaymentThatIsUnderfunded(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildFormationResultMetaXDR(txbuildtest.FormationResultMetaParams{
			InitiatorSigner: remoteSigner.Address(),
			ResponderSigner: localSigner.Address(),
			InitiatorEscrow: remoteEscrowAccount.Address(),
			ResponderEscrow: localEscrowAccount.Address(),
			StartSequence:   101,
			Asset:           txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = responderChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// A close agreement from the remote participant should be rejected if the
	// payment changes the balance in the favor of the remote.
	responderChannel.latestAuthorizedCloseAgreement = CloseAgreement{
		Details: CloseAgreementDetails{
			IterationNumber:            1,
			Balance:                    -60, // Local (responder) owes remote (initiator) 60.
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
			ConfirmingSigner:           localSigner.FromAddress(),
		},
	}

	_, err := responderChannel.ProposePayment(110)
	assert.EqualError(t, err, "amount over commits: account is underfunded to make payment")
	assert.ErrorIs(t, err, ErrUnderfunded)

	// The same close payment should pass if the balance has been updated.
	responderChannel.UpdateLocalEscrowAccountBalance(200)
	_, err = responderChannel.ProposePayment(110)
	assert.NoError(t, err)
}

func TestLastConfirmedPayment(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()
	sendingChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})
	receiverChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := sendingChannel.ProposeOpen(OpenParams{
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now().Add(5 * time.Minute),
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
			StartingSequence:           101,
		})
		require.NoError(t, err)
		m, err = receiverChannel.ConfirmOpen(m)
		require.NoError(t, err)
		_, err = sendingChannel.ConfirmOpen(m)
		require.NoError(t, err)

		ftx, err := sendingChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildFormationResultMetaXDR(txbuildtest.FormationResultMetaParams{
			InitiatorSigner: localSigner.Address(),
			ResponderSigner: remoteSigner.Address(),
			InitiatorEscrow: localEscrowAccount.Address(),
			ResponderEscrow: remoteEscrowAccount.Address(),
			StartSequence:   101,
			Asset:           txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = sendingChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = receiverChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := sendingChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		cs, err = receiverChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	sendingChannel.UpdateLocalEscrowAccountBalance(1000)
	sendingChannel.UpdateRemoteEscrowAccountBalance(1000)
	receiverChannel.UpdateLocalEscrowAccountBalance(1000)
	receiverChannel.UpdateRemoteEscrowAccountBalance(1000)

	// Test the returned close agreemenets are as expected.
	ca, err := sendingChannel.ProposePayment(200)
	require.NoError(t, err)
	assert.Equal(t, ca, sendingChannel.latestUnauthorizedCloseAgreement)

	caResponse, err := receiverChannel.ConfirmPayment(ca)
	require.NoError(t, err)
	assert.Equal(t, caResponse, receiverChannel.latestAuthorizedCloseAgreement)

	// Confirming a close agreement with same sequence number but different Amount should error
	caDifferent := CloseAgreement{
		Details: CloseAgreementDetails{
			IterationNumber:            2,
			Balance:                    400,
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
		},
		ProposerSignatures:  ca.ProposerSignatures,
		ConfirmerSignatures: ca.ConfirmerSignatures,
	}
	_, err = sendingChannel.ConfirmPayment(caDifferent)
	require.EqualError(t, err, "validating payment: close agreement does not match the close agreement already in progress")
	assert.Equal(t, ca, sendingChannel.latestUnauthorizedCloseAgreement)

	// Confirming a payment with same sequence number and same amount should pass
	caFinal, err := sendingChannel.ConfirmPayment(caResponse)
	require.NoError(t, err)
	assert.Equal(t, CloseAgreement{}, sendingChannel.latestUnauthorizedCloseAgreement)
	assert.Equal(t, caFinal, sendingChannel.latestAuthorizedCloseAgreement)
	assert.Equal(t, caFinal, caResponse)
}

func TestChannel_ProposeAndConfirmPayment_rejectIfChannelNotOpen(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	senderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		MaxOpenExpiry:       10 * time.Second,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})
	receiverChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		MaxOpenExpiry:       10 * time.Second,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
	})

	// Before open, proposing a payment should error.
	_, err := senderChannel.ProposePayment(10)
	require.EqualError(t, err, "cannot propose a payment before channel is opened")

	// Before open, confirming a payment should error.
	_, err = senderChannel.ConfirmPayment(CloseAgreement{})
	require.EqualError(t, err, "validating payment: cannot confirm a payment before channel is opened")

	// Open channel.
	m, err := senderChannel.ProposeOpen(OpenParams{
		Asset:                      NativeAsset,
		ExpiresAt:                  time.Now().Add(5 * time.Second),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
		StartingSequence:           101,
	})
	require.NoError(t, err)
	m, err = receiverChannel.ConfirmOpen(m)
	require.NoError(t, err)
	_, err = senderChannel.ConfirmOpen(m)
	require.NoError(t, err)

	// Before an open is executed and validated, proposing and confirming a payment should error.
	assert.False(t, senderChannel.latestAuthorizedCloseAgreement.isEmpty())
	_, err = senderChannel.ProposePayment(10)
	require.EqualError(t, err, "cannot propose a payment before channel is opened")

	_, err = senderChannel.ConfirmPayment(CloseAgreement{})
	require.EqualError(t, err, "validating payment: cannot confirm a payment before channel is opened")

	// Put channel into the Open state.
	{
		ftx, err := senderChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildFormationResultMetaXDR(txbuildtest.FormationResultMetaParams{
			InitiatorSigner: localSigner.Address(),
			ResponderSigner: remoteSigner.Address(),
			InitiatorEscrow: localEscrowAccount.Address(),
			ResponderEscrow: remoteEscrowAccount.Address(),
			StartSequence:   101,
			Asset:           txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = senderChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = receiverChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := senderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		cs, err = receiverChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// Sender proposes coordinated close.
	ca, err := senderChannel.ProposeClose()
	require.NoError(t, err)

	// After proposing a coordinated close, proposing a payment should error.
	_, err = senderChannel.ProposePayment(10)
	require.EqualError(t, err, "cannot propose payment after proposing a coordinated close")

	// After proposing a coordinated close, confirming a payment should error.
	p := CloseAgreement{
		Details: CloseAgreementDetails{
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
			IterationNumber:            1,
			Balance:                    0,
			ConfirmingSigner:           localSigner.FromAddress(),
		},
	}
	_, err = senderChannel.ConfirmPayment(p)
	require.EqualError(t, err, "validating payment: cannot confirm payment after proposing a coordinated close")

	// Finish close.
	ca, err = receiverChannel.ConfirmClose(ca)
	require.NoError(t, err)
	_, err = senderChannel.ConfirmClose(ca)
	require.NoError(t, err)

	// After a confirmed coordinated close, proposing a payment should error.
	_, err = senderChannel.ProposePayment(10)
	require.EqualError(t, err, "cannot propose payment after an accepted coordinated close")

	_, err = receiverChannel.ProposePayment(10)
	require.EqualError(t, err, "cannot propose payment after an accepted coordinated close")

	// After a confirmed coordinated close, confirming a payment should error.
	p = CloseAgreement{
		Details: CloseAgreementDetails{
			ObservationPeriodTime:      0,
			ObservationPeriodLedgerGap: 0,
			IterationNumber:            2,
			Balance:                    10,
			ConfirmingSigner:           localSigner.FromAddress(),
		},
	}
	_, err = receiverChannel.ConfirmPayment(p)
	require.EqualError(t, err, "validating payment: cannot confirm payment after an accepted coordinated close")

	_, err = senderChannel.ConfirmPayment(p)
	require.EqualError(t, err, "validating payment: cannot confirm payment after an accepted coordinated close")
}

func TestChannel_enforceOnlyOneCloseAgreementAllowed(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	senderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		MaxOpenExpiry:       10 * time.Second,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})
	receiverChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		MaxOpenExpiry:       10 * time.Second,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
	})

	// Open channel.
	m, err := senderChannel.ProposeOpen(OpenParams{
		Asset:                      NativeAsset,
		ExpiresAt:                  time.Now().Add(5 * time.Second),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
		StartingSequence:           101,
	})
	require.NoError(t, err)
	m, err = receiverChannel.ConfirmOpen(m)
	require.NoError(t, err)
	_, err = senderChannel.ConfirmOpen(m)
	require.NoError(t, err)

	// Put channel into the Open state.
	{
		ftx, err := senderChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildFormationResultMetaXDR(txbuildtest.FormationResultMetaParams{
			InitiatorSigner: localSigner.Address(),
			ResponderSigner: remoteSigner.Address(),
			InitiatorEscrow: localEscrowAccount.Address(),
			ResponderEscrow: remoteEscrowAccount.Address(),
			StartSequence:   101,
			Asset:           txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = senderChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = receiverChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := senderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		cs, err = receiverChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	senderChannel.UpdateLocalEscrowAccountBalance(1000)
	senderChannel.UpdateRemoteEscrowAccountBalance(1000)

	caOriginal := senderChannel.latestAuthorizedCloseAgreement

	// sender proposes payment
	_, err = senderChannel.ProposePayment(10)
	require.NoError(t, err)

	ucaOriginal := senderChannel.latestUnauthorizedCloseAgreement

	// sender should not be able to propose a second payment until the first is finished
	_, err = senderChannel.ProposePayment(20)
	require.EqualError(t, err, "cannot start a new payment while an unfinished one exists")

	// sender should not be able to propose coordinated close while unfinished payment exists
	_, err = senderChannel.ProposeClose()
	require.EqualError(t, err, "cannot propose coordinated close while an unfinished payment exists")

	// sender should still have the original close agreement
	require.Equal(t, senderChannel.latestAuthorizedCloseAgreement, caOriginal)

	// sender should still have the latestUnauthorizedCloseAgreement
	require.Equal(t, senderChannel.latestUnauthorizedCloseAgreement, ucaOriginal)
}

func TestChannel_ConfirmPayment_validatePaymentAmount(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now().Add(5 * time.Minute),
			StartingSequence:           101,
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildFormationResultMetaXDR(txbuildtest.FormationResultMetaParams{
			InitiatorSigner: localSigner.Address(),
			ResponderSigner: remoteSigner.Address(),
			InitiatorEscrow: localEscrowAccount.Address(),
			ResponderEscrow: remoteEscrowAccount.Address(),
			StartSequence:   101,
			Asset:           txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = responderChannel.IngestTx(ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		cs, err = responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}
	initiatorChannel.UpdateLocalEscrowAccountBalance(200)
	initiatorChannel.UpdateRemoteEscrowAccountBalance(200)

	responderChannel.UpdateLocalEscrowAccountBalance(200)
	responderChannel.UpdateRemoteEscrowAccountBalance(200)

	// Initiator proposes payment to Responder.
	ca, err := initiatorChannel.ProposePayment(50)
	require.NoError(t, err)
	assert.Equal(t, int64(50), ca.Details.Balance)
	assert.Equal(t, int64(50), ca.Details.PaymentAmount)

	// An incorrect PaymentAmount should error.
	ca.Details.PaymentAmount = 49
	_, err = responderChannel.ConfirmPayment(ca)
	require.EqualError(t, err, "validating payment: close agreement payment amount is unexpected: "+
		"current balance: 0 proposed balance: 50 payment amount: 49 initiator proposed: true")

	ca.Details.PaymentAmount = 50
	ca, err = responderChannel.ConfirmPayment(ca)
	require.NoError(t, err)
	ca, err = initiatorChannel.ConfirmPayment(ca)
	require.NoError(t, err)

	// Responder proposes payment to Initiator.
	ca, err = responderChannel.ProposePayment(100)
	require.NoError(t, err)
	assert.Equal(t, int64(-50), ca.Details.Balance)
	assert.Equal(t, int64(100), ca.Details.PaymentAmount)

	// An incorrect Balance should error.
	ca.Details.Balance = -49
	_, err = initiatorChannel.ConfirmPayment(ca)
	require.EqualError(t, err, "validating payment: close agreement payment amount is unexpected: "+
		"current balance: 50 proposed balance: -49 payment amount: 100 initiator proposed: false")

	ca.Details.Balance = -50
	ca, err = initiatorChannel.ConfirmPayment(ca)
	require.NoError(t, err)
}
