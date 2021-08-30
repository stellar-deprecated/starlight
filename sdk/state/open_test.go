package state

import (
	"strconv"
	"testing"
	"time"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAgreement_Equal(t *testing.T) {
	kp := keypair.MustRandom().FromAddress()
	testCases := []struct {
		oa1       OpenAgreement
		oa2       OpenAgreement
		wantEqual bool
	}{
		{OpenAgreement{}, OpenAgreement{}, true},
		{
			OpenAgreement{
				Details: OpenAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
			},
			OpenAgreement{
				Details: OpenAgreementDetails{
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
			OpenAgreement{
				Details: OpenAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
			},
			OpenAgreement{},
			false,
		},
		{
			OpenAgreement{
				Details: OpenAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
				ProposerSignatures: OpenAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			OpenAgreement{
				Details: OpenAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
				ProposerSignatures: OpenAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			true,
		},
		{
			OpenAgreement{
				Details: OpenAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
				ProposerSignatures: OpenAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			OpenAgreement{},
			false,
		},
		{
			OpenAgreement{
				Details: OpenAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
				ProposerSignatures: OpenAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			OpenAgreement{
				Details: OpenAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
				ProposerSignatures: OpenAgreementSignatures{
					Close: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			false,
		},
		{
			OpenAgreement{
				Details: OpenAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           kp,
				},
				ProposerSignatures: OpenAgreementSignatures{
					Close: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			OpenAgreement{
				Details: OpenAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
					ConfirmingSigner:           keypair.MustRandom().FromAddress(),
				},
				ProposerSignatures: OpenAgreementSignatures{
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
	localEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(101),
	}
	remoteEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(202),
	}
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
	localEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(101),
	}
	remoteEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(202),
	}

	channel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})
	channel.openAgreement = OpenAgreement{
		Details: OpenAgreementDetails{
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 1,
			Asset:                      NativeAsset,
		},
	}

	oa := OpenAgreementDetails{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		Asset:                      NativeAsset,
	}

	{
		// invalid ObservationPeriodTime
		d := oa
		d.ObservationPeriodTime = 0
		_, err := channel.ConfirmOpen(OpenAgreement{Details: d})
		require.EqualError(t, err, "validating open agreement: input open agreement details do not match the saved open agreement details")
	}

	{
		// invalid different asset
		d := oa
		d.Asset = "ABC:GCDFU7RNY6HTYQKP7PYHBMXXKXZ4HET6LMJ5CDO7YL5NMYH4T2BSZCPZ"
		_, err := channel.ConfirmOpen(OpenAgreement{Details: d})
		require.EqualError(t, err, "validating open agreement: input open agreement details do not match the saved open agreement details")
	}
}

func TestConfirmOpen_rejectsOpenAgreementsWithLongFormations(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(101),
	}
	remoteEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(202),
	}

	channel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       10 * time.Second,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})

	_, err := channel.ConfirmOpen(OpenAgreement{Details: OpenAgreementDetails{
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
	localEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(101),
	}
	remoteEscrowAccount := &EscrowAccount{
		Address:        keypair.MustRandom().FromAddress(),
		SequenceNumber: int64(202),
	}

	channel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})
	channel.openAgreement = OpenAgreement{
		Details: OpenAgreementDetails{
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 1,
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now(),
			ProposingSigner:            localSigner.FromAddress(),
			ConfirmingSigner:           remoteSigner.FromAddress(),
		},
		ProposerSignatures: OpenAgreementSignatures{
			Declaration: xdr.Signature{0},
			Close:       xdr.Signature{1},
			Formation:   xdr.Signature{2},
		},
		ConfirmerSignatures: OpenAgreementSignatures{
			Declaration: xdr.Signature{3},
			Close:       xdr.Signature{4},
			Formation:   xdr.Signature{5},
		},
	}

	declTx, closeTx, _, err := channel.openTxs(channel.openAgreement.Details)
	require.NoError(t, err)
	formationTx, err := channel.OpenTx()
	require.NoError(t, err)
	declTxHash, err := declTx.Hash(channel.networkPassphrase)
	require.NoError(t, err)
	closeTxHash, err := closeTx.Hash(channel.networkPassphrase)
	require.NoError(t, err)
	// TODO: Compare the non-signature parts of formationTx with the result of
	// channel.openTx() when there is an practical way of doing that added to
	// txnbuild.
	assert.ElementsMatch(t, []xdr.DecoratedSignature{
		{Hint: localSigner.Hint(), Signature: []byte{2}},
		{Hint: remoteSigner.Hint(), Signature: []byte{5}},
		xdr.NewDecoratedSignatureForPayload([]byte{3}, remoteSigner.Hint(), declTxHash[:]),
		xdr.NewDecoratedSignatureForPayload([]byte{4}, remoteSigner.Hint(), closeTxHash[:]),
	}, formationTx.Signatures())
}

func TestChannel_OpenAgreementIsFull(t *testing.T) {
	oa := OpenAgreement{}
	assert.False(t, oa.isFull())

	oa = OpenAgreement{
		ProposerSignatures: OpenAgreementSignatures{
			Close:       xdr.Signature{1},
			Declaration: xdr.Signature{1},
			Formation:   xdr.Signature{1},
		},
	}
	assert.False(t, oa.isFull())

	oa.ConfirmerSignatures = OpenAgreementSignatures{
		Close:       xdr.Signature{1},
		Declaration: xdr.Signature{1},
	}
	assert.False(t, oa.isFull())

	oa.ConfirmerSignatures.Formation = xdr.Signature{1}
	assert.True(t, oa.isFull())
}

func TestChannel_ProposeAndConfirmOpen_rejectIfChannelAlreadyOpen(t *testing.T) {
	initiatorSigner, err := keypair.ParseFull("SCBMAMOPWKL2YHWELK63VLAY2R74A6GTLLD4ON223B7K5KZ37MUR6IDF")
	require.NoError(t, err)
	responderSigner, err := keypair.ParseFull("SBM7D2IIDSRX5Y3VMTMTXXPB6AIB4WYGZBC2M64U742BNOK32X6SW4NF")
	require.NoError(t, err)

	initiatorEscrow, err := keypair.ParseAddress("GAU4CFXQI6HLK5PPY2JWU3GMRJIIQNLF24XRAHX235F7QTG6BEKLGQ36")
	require.NoError(t, err)
	responderEscrow, err := keypair.ParseAddress("GBQNGSEHTFC4YGQ3EXHIL7JQBA6265LFANKFFAYKHM7JFGU5CORROEGO")
	require.NoError(t, err)

	initiatorEscrowAccount := &EscrowAccount{
		Address:        initiatorEscrow.FromAddress(),
		SequenceNumber: int64(28037546508288),
	}

	responderEscrowAccount := &EscrowAccount{
		Address:        responderEscrow.FromAddress(),
		SequenceNumber: int64(28054726377472),
	}
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           true,
		LocalSigner:         initiatorSigner,
		RemoteSigner:        responderSigner.FromAddress(),
		LocalEscrowAccount:  initiatorEscrowAccount,
		RemoteEscrowAccount: responderEscrowAccount,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		MaxOpenExpiry:       time.Hour,
		Initiator:           false,
		LocalSigner:         responderSigner,
		RemoteSigner:        initiatorSigner.FromAddress(),
		LocalEscrowAccount:  responderEscrowAccount,
		RemoteEscrowAccount: initiatorEscrowAccount,
	})

	open, err := initiatorChannel.ProposeOpen((OpenParams{
		Asset:     NativeAsset,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}))
	require.NoError(t, err)
	open, err = responderChannel.ConfirmOpen(open)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(open)
	require.NoError(t, err)

	formationTx, err := initiatorChannel.OpenTx()
	require.NoError(t, err)
	formationTxXDR, err := formationTx.Base64()
	require.NoError(t, err)

	validResultXDR := "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA="
	resultMetaXDR := "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"
	err = initiatorChannel.IngestTx(formationTxXDR, validResultXDR, resultMetaXDR)
	require.NoError(t, err)

	cs, err := initiatorChannel.State()
	require.NoError(t, err)
	assert.Equal(t, StateOpen, cs)

	// local channel trying to open channel again should error.
	_, err = initiatorChannel.ProposeOpen((OpenParams{
		Asset:     NativeAsset,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}))
	require.EqualError(t, err, "cannot propose a new open if channel has already opened")

	// local channel trying to confirm an open again should error.
	_, err = initiatorChannel.ConfirmOpen(open)
	require.EqualError(t, err, "validating open agreement: cannot confirm a new open if channel is already opened")
}
