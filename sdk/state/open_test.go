package state

import (
	"strconv"
	"testing"
	"time"

	"github.com/stellar/experimental-payment-channels/sdk/txbuildtest"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAgreement_Equal(t *testing.T) {
	kp := keypair.MustRandom().FromAddress()
	testCases := []struct {
		oa1       OpenEnvelope
		oa2       OpenEnvelope
		wantEqual bool
	}{
		{OpenEnvelope{}, OpenEnvelope{}, true},
		{
			OpenEnvelope{
				Details: OpenDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
			},
			OpenEnvelope{
				Details: OpenDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
			},
			true,
		},
		{
			OpenEnvelope{
				Details: OpenDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
			},
			OpenEnvelope{},
			false,
		},
		{
			OpenEnvelope{
				Details: OpenDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
				ProposerSignatures: OpenSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			OpenEnvelope{
				Details: OpenDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
				ProposerSignatures: OpenSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			true,
		},
		{
			OpenEnvelope{
				Details: OpenDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
				ProposerSignatures: OpenSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			OpenEnvelope{},
			false,
		},
		{
			OpenEnvelope{
				Details: OpenDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
				ProposerSignatures: OpenSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			OpenEnvelope{
				Details: OpenDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
				ProposerSignatures: OpenSignatures{
					Close: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			false,
		},
		{
			OpenEnvelope{
				Details: OpenDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
				ProposerSignatures: OpenSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			OpenEnvelope{
				Details: OpenDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           keypair.MustRandom().FromAddress(),
				},
				ProposerSignatures: OpenSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			false,
		},
	}
	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			equal := tc.oa1.Equal(tc.oa2)
			assert.Equal(t, tc.wantEqual, equal)
		})
	}
}

func TestProposeOpen_validAsset(t *testing.T) {
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
	})

	_, err := sendingChannel.ProposeOpen(OpenParams{
		Asset:     NativeAsset,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	})
	require.NoError(t, err)

	// TODO(leighmcculloch): Bring this test back in a future PR.
	// _, err = sendingChannel.ProposeOpen(OpenParams{
	// 	Asset:     ":GCSZIQEYTDI427C2XCCIWAGVHOIZVV2XKMRELUTUVKOODNZWSR2OLF6P",
	// 	ExpiresAt: time.Now().Add(5 * time.Minute),
	// })
	// require.EqualError(t, err, `validation failed for *txnbuild.ChangeTrust operation: Field: Line, Error: asset code length must be between 1 and 12 characters`)

	// TODO(leighmcculloch): Bring this test back in a future PR.
	// _, err = sendingChannel.ProposeOpen(OpenParams{
	// 	Asset:     "ABCD:GABCD:AB",
	// 	ExpiresAt: time.Now().Add(5 * time.Minute),
	// })
	// require.EqualError(t, err, `validation failed for *txnbuild.ChangeTrust operation: Field: Line, Error: asset issuer: GABCD:AB is not a valid stellar public key`)

	_, err = sendingChannel.ProposeOpen(OpenParams{
		Asset:     "ABCD:GCSZIQEYTDI427C2XCCIWAGVHOIZVV2XKMRELUTUVKOODNZWSR2OLF6P",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	})
	require.NoError(t, err)
}

func TestConfirmOpen_rejectsDifferentOpenAgreements(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	channel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})
	channel.openAgreement = OpenAgreement{
		Envelope: OpenEnvelope{
			Details: OpenDetails{
				ObservationPeriodTime:      1,
				ObservationPeriodLedgerGap: 1,
				Asset:                      NativeAsset,
			},
		},
	}

	oa := OpenDetails{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		Asset:                      NativeAsset,
	}

	{
		// invalid ObservationPeriodTime
		d := oa
		d.ObservationPeriodTime = 0
		_, err := channel.ConfirmOpen(OpenEnvelope{Details: d})
		require.EqualError(t, err, "validating open agreement: input open agreement details do not match the saved open agreement details")
	}

	{
		// invalid different asset
		d := oa
		d.Asset = "ABC:GCDFU7RNY6HTYQKP7PYHBMXXKXZ4HET6LMJ5CDO7YL5NMYH4T2BSZCPZ"
		_, err := channel.ConfirmOpen(OpenEnvelope{Details: d})
		require.EqualError(t, err, "validating open agreement: input open agreement details do not match the saved open agreement details")
	}
}

func TestConfirmOpen_rejectsOpenAgreementsWithLongFormations(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	channel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       10 * time.Second,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})

	_, err := channel.ConfirmOpen(OpenEnvelope{Details: OpenDetails{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		Asset:                      NativeAsset,
		ExpiresAt:                  time.Now().Add(100 * time.Second),
	}})
	require.EqualError(t, err, "validating open agreement: input open agreement expire too far into the future")
}

func TestChannel_OpenTx(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	channel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})
	oe := OpenEnvelope{
		Details: OpenDetails{
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 1,
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now(),
			ProposingSigner:            localSigner.FromAddress(),
			ConfirmingSigner:           remoteSigner.FromAddress(),
		},
		ProposerSignatures: OpenSignatures{
			Declaration: xdr.Signature{0},
			Close:       xdr.Signature{1},
			Formation:   xdr.Signature{2},
		},
		ConfirmerSignatures: OpenSignatures{
			Declaration: xdr.Signature{3},
			Close:       xdr.Signature{4},
			Formation:   xdr.Signature{5},
		},
	}
	txs, closeTxs, err := channel.openTxs(oe.Details)
	require.NoError(t, err)
	channel.openAgreement = OpenAgreement{Envelope: oe, Transactions: txs, CloseTransactions: closeTxs}
	declTxHash := closeTxs.DeclarationHash
	closeTxHash := closeTxs.CloseHash

	// TODO: Compare the non-signature parts of formationTx with the result of
	// channel.openTx() when there is an practical way of doing that added to
	// txnbuild.

	// Check signatures are populated.
	formationTx, err := channel.OpenTx()
	require.NoError(t, err)
	assert.ElementsMatch(t, []xdr.DecoratedSignature{
		{Hint: localSigner.Hint(), Signature: []byte{2}},
		{Hint: remoteSigner.Hint(), Signature: []byte{5}},
		xdr.NewDecoratedSignatureForPayload([]byte{3}, remoteSigner.Hint(), declTxHash[:]),
		xdr.NewDecoratedSignatureForPayload([]byte{4}, remoteSigner.Hint(), closeTxHash[:]),
	}, formationTx.Signatures())

	// Check stored txs are used by replacing the stored tx with an identifiable
	// tx and checking that's what is used.
	testTx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{AccountID: localEscrowAccount.Address(), Sequence: 123456789},
		BaseFee:       txnbuild.MinBaseFee,
		Timebounds:    txnbuild.NewInfiniteTimeout(),
		Operations:    []txnbuild.Operation{&txnbuild.BumpSequence{}},
	})
	require.NoError(t, err)
	channel.openAgreement.Transactions = OpenTransactions{
		Formation: testTx,
	}
	formationTx, err = channel.OpenTx()
	require.NoError(t, err)
	assert.Equal(t, int64(123456789), formationTx.SequenceNumber())
}

func TestChannel_OpenAgreementIsFull(t *testing.T) {
	oa := OpenEnvelope{}
	assert.False(t, oa.isFull())

	oa = OpenEnvelope{
		ProposerSignatures: OpenSignatures{
			Close:       xdr.Signature{1},
			Declaration: xdr.Signature{1},
			Formation:   xdr.Signature{1},
		},
	}
	assert.False(t, oa.isFull())

	oa.ConfirmerSignatures = OpenSignatures{
		Close:       xdr.Signature{1},
		Declaration: xdr.Signature{1},
	}
	assert.False(t, oa.isFull())

	oa.ConfirmerSignatures.Formation = xdr.Signature{1}
	assert.True(t, oa.isFull())
}

func TestChannel_ProposeAndConfirmOpen_rejectIfChannelAlreadyOpen(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

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

	// Open channel.
	m, err := initiatorChannel.ProposeOpen(OpenParams{
		Asset:                      NativeAsset,
		ExpiresAt:                  time.Now().Add(5 * time.Second),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
		StartingSequence:           101,
	})
	require.NoError(t, err)
	m, err = responderChannel.ConfirmOpen(m.Envelope)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(m.Envelope)
	require.NoError(t, err)

	{
		// Ingest the formationTx successfully to enter the Open state.
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

	_, err = initiatorChannel.ProposeOpen(OpenParams{
		Asset:                      NativeAsset,
		ExpiresAt:                  time.Now().Add(5 * time.Second),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
	})
	require.EqualError(t, err, "cannot propose a new open if channel is already opened")

	_, err = responderChannel.ConfirmOpen(m.Envelope)
	require.EqualError(t, err, "validating open agreement: cannot confirm a new open if channel is already opened")

	// A channel without a full open agreement should be able to propose an open
	initiatorChannel.openAgreement.Envelope.ConfirmerSignatures = OpenSignatures{}
	_, err = initiatorChannel.ProposeOpen(OpenParams{
		Asset:     NativeAsset,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	})
	require.NoError(t, err)
}
