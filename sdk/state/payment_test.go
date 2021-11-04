package state

import (
	"strconv"
	"testing"
	"time"

	fuzz "github.com/google/gofuzz"
	"github.com/stellar/experimental-payment-channels/sdk/txbuildtest"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloseDetails_Equal(t *testing.T) {
	assert.True(t, CloseDetails{}.Equal(CloseDetails{}))

	// The same value should be equal.
	ft := time.Now().UnixNano()
	od1 := CloseDetails{}
	fuzz.NewWithSeed(ft).NilChance(0).Fuzz(&od1)
	t.Log("od1:", od1)
	od2 := CloseDetails{}
	fuzz.NewWithSeed(ft).NilChance(0).Fuzz(&od2)
	t.Log("od2:", od2)
	assert.True(t, od1.Equal(od2))

	// Different values should never be equal.
	for i := 0; i < 20; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			f := fuzz.New()
			a := CloseSignatures{}
			f.Fuzz(&a)
			t.Log("a:", a)
			b := CloseSignatures{}
			f.Fuzz(&b)
			t.Log("b:", b)
			assert.False(t, a.Equal(b))
			assert.False(t, b.Equal(a))
		})
	}
}

func TestCloseSignatures_Equal(t *testing.T) {
	assert.True(t, CloseSignatures{}.Equal(CloseSignatures{}))

	// The same value should be equal. It is common for CloseSignatures to be
	// defined in whole or not at all, so we test that use case.
	ft := time.Now().UnixNano()
	os1 := CloseSignatures{}
	fuzz.NewWithSeed(ft).NilChance(0).Fuzz(&os1)
	t.Log("os1:", os1)
	os2 := CloseSignatures{}
	fuzz.NewWithSeed(ft).NilChance(0).Fuzz(&os2)
	t.Log("os2:", os2)
	assert.True(t, os1.Equal(os2))

	// Different values should never be equal.
	for i := 0; i < 20; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			f := fuzz.New()
			a := CloseSignatures{}
			f.Fuzz(&a)
			t.Log("a:", a)
			b := CloseSignatures{}
			f.Fuzz(&b)
			t.Log("b:", b)
			assert.False(t, a.Equal(b))
			assert.False(t, b.Equal(a))
		})
	}
}

func TestCloseEnvelope_Equal(t *testing.T) {
	assert.True(t, CloseEnvelope{}.Equal(CloseEnvelope{}))

	// The same value should be equal. It's common for CloseEnvelopes to start
	// with details then have signatures added, so we check that pattern of
	// incrementally adding fields.
	f := fuzz.New().NilChance(0)
	od := CloseDetails{}
	f.Fuzz(&od)
	t.Log("od:", od)
	ps := CloseSignatures{}
	f.Fuzz(&ps)
	t.Log("ps:", ps)
	cs := CloseSignatures{}
	f.Fuzz(&cs)
	t.Log("cs:", cs)
	assert.True(t, CloseEnvelope{Details: od}.Equal(CloseEnvelope{Details: od}))
	assert.True(t, CloseEnvelope{Details: od, ProposerSignatures: ps}.Equal(CloseEnvelope{Details: od, ProposerSignatures: ps}))
	assert.True(t, CloseEnvelope{Details: od, ProposerSignatures: ps, ConfirmerSignatures: cs}.Equal(CloseEnvelope{Details: od, ProposerSignatures: ps, ConfirmerSignatures: cs}))

	// Different values should never be equal.
	for i := 0; i < 20; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			f := fuzz.New()
			a := CloseEnvelope{}
			f.Fuzz(&a)
			t.Log("a:", a)
			b := CloseEnvelope{}
			f.Fuzz(&b)
			t.Log("b:", b)
			assert.False(t, a.Equal(b))
			assert.False(t, b.Equal(a))
		})
	}
}

func TestChannel_ConfirmPayment_acceptsSameObservationPeriod(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   localSigner.Address(),
			ResponderSigner:   remoteSigner.Address(),
			InitiatorMultiSig: localMultiSigAccount.Address(),
			ResponderMultiSig: remoteMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// A close agreement from the remote participant should be accepted if the
	// observation period matches the channels observation period.
	{
		initiatorChannel.latestAuthorizedCloseAgreement = CloseAgreement{
			Envelope: CloseEnvelope{
				Details: CloseDetails{
					ObservationPeriodTime:      1,
					ObservationPeriodLedgerGap: 1,
					ConfirmingSigner:           localSigner.FromAddress(),
				},
			},
		}

		txs, err := initiatorChannel.closeTxs(initiatorChannel.openAgreement.Envelope.Details, CloseDetails{
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
		_, err = initiatorChannel.ConfirmPayment(CloseEnvelope{
			Details: CloseDetails{
				IterationNumber:            1,
				ObservationPeriodTime:      1,
				ObservationPeriodLedgerGap: 1,
				ProposingSigner:            remoteSigner.FromAddress(),
				ConfirmingSigner:           localSigner.FromAddress(),
			},
			ProposerSignatures: CloseSignatures{
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
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   localSigner.Address(),
			ResponderSigner:   remoteSigner.Address(),
			InitiatorMultiSig: localMultiSigAccount.Address(),
			ResponderMultiSig: remoteMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	initiatorChannel.latestAuthorizedCloseAgreement = CloseAgreement{
		Envelope: CloseEnvelope{
			Details: CloseDetails{
				ObservationPeriodTime:      1,
				ObservationPeriodLedgerGap: 1,
				ConfirmingSigner:           localSigner.FromAddress(),
			},
		},
	}

	// A close agreement from the remote participant should be rejected if the
	// observation period doesn't match the channels observation period.
	{
		txs, err := initiatorChannel.closeTxs(initiatorChannel.openAgreement.Envelope.Details, CloseDetails{
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
		_, err = initiatorChannel.ConfirmPayment(CloseEnvelope{
			Details: CloseDetails{
				IterationNumber:            1,
				ObservationPeriodTime:      0,
				ObservationPeriodLedgerGap: 0,
			},
			ProposerSignatures: CloseSignatures{
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
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   localSigner.Address(),
			ResponderSigner:   remoteSigner.Address(),
			InitiatorMultiSig: localMultiSigAccount.Address(),
			ResponderMultiSig: remoteMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// A close agreement from the remote participant should be rejected if the
	// payment changes the balance in the favor of the remote.
	initiatorChannel.latestAuthorizedCloseAgreement = CloseAgreement{
		Envelope: CloseEnvelope{
			Details: CloseDetails{
				IterationNumber:            1,
				Balance:                    100, // Local (initiator) owes remote (responder) 100.
				ObservationPeriodTime:      10,
				ObservationPeriodLedgerGap: 10,
				ConfirmingSigner:           localSigner.FromAddress(),
			},
		},
	}

	ca := CloseDetails{
		IterationNumber:            2,
		Balance:                    110, // Local (initiator) owes remote (responder) 110, payment of 10 from ❌ local to remote.
		PaymentAmount:              -10, // Not possible to have a negative payment amount, but hardcode to test this validation.
		ProposingSigner:            remoteSigner.FromAddress(),
		ConfirmingSigner:           localSigner.FromAddress(),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
	}
	txs, err := initiatorChannel.closeTxs(initiatorChannel.openAgreement.Envelope.Details, ca)
	txDecl := txs.Declaration
	txClose := txs.Close
	require.NoError(t, err)
	txDecl, err = txDecl.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	txClose, err = txClose.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmPayment(CloseEnvelope{
		Details: ca,
		ProposerSignatures: CloseSignatures{
			Declaration: txDecl.Signatures()[0].Signature,
			Close:       txClose.Signatures()[0].Signature,
		},
	})
	require.EqualError(t, err, "close agreement is a payment to the proposer")
}

func TestChannel_ConfirmPayment_localWhoIsResponderRejectsPaymentToRemoteWhoIsInitiator(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   remoteSigner.Address(),
			ResponderSigner:   localSigner.Address(),
			InitiatorMultiSig: remoteMultiSigAccount.Address(),
			ResponderMultiSig: localMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = responderChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// A close agreement from the remote participant should be rejected if the
	// payment changes the balance in the favor of the remote.
	responderChannel.latestAuthorizedCloseAgreement = CloseAgreement{
		Envelope: CloseEnvelope{
			Details: CloseDetails{
				IterationNumber:            1,
				Balance:                    100, // Remote (initiator) owes local (responder) 100.
				ObservationPeriodTime:      10,
				ObservationPeriodLedgerGap: 10,
				ConfirmingSigner:           localSigner.FromAddress(),
			},
		},
	}
	ca := CloseDetails{
		IterationNumber:            2,
		Balance:                    90,  // Remote (initiator) owes local (responder) 90, payment of 10 from ❌ local to remote.
		PaymentAmount:              -10, // Not possible to have a negative payment amount, but hardcode to test this validation.
		ProposingSigner:            remoteSigner.FromAddress(),
		ConfirmingSigner:           localSigner.FromAddress(),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
	}

	txs, err := responderChannel.closeTxs(responderChannel.openAgreement.Envelope.Details, ca)
	txDecl := txs.Declaration
	txClose := txs.Close
	require.NoError(t, err)
	txDecl, err = txDecl.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	txClose, err = txClose.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	_, err = responderChannel.ConfirmPayment(CloseEnvelope{
		Details: ca,
		ProposerSignatures: CloseSignatures{
			Declaration: txDecl.Signatures()[0].Signature,
			Close:       txClose.Signatures()[0].Signature,
		},
	})
	require.EqualError(t, err, "close agreement is a payment to the proposer")
}

func TestChannel_ConfirmPayment_initiatorRejectsPaymentThatIsUnderfunded(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   localSigner.Address(),
			ResponderSigner:   remoteSigner.Address(),
			InitiatorMultiSig: localMultiSigAccount.Address(),
			ResponderMultiSig: remoteMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// A close agreement from the remote participant should be rejected if the
	// payment changes the balance in the favor of the remote.
	initiatorChannel.latestAuthorizedCloseAgreement = CloseAgreement{
		Envelope: CloseEnvelope{
			Details: CloseDetails{
				IterationNumber:            1,
				Balance:                    -60, // Remote (responder) owes local (initiator) 60.
				ObservationPeriodTime:      10,
				ObservationPeriodLedgerGap: 10,
				ConfirmingSigner:           localSigner.FromAddress(),
			},
		},
	}

	ca := CloseDetails{
		IterationNumber:            2,
		Balance:                    -110, // Remote (responder) owes local (initiator) 110, which responder ❌ cannot pay.
		PaymentAmount:              50,
		ProposingSigner:            remoteSigner.FromAddress(),
		ConfirmingSigner:           localSigner.FromAddress(),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
	}
	txs, err := initiatorChannel.closeTxs(initiatorChannel.openAgreement.Envelope.Details, ca)
	txDecl := txs.Declaration
	txClose := txs.Close
	require.NoError(t, err)
	txDecl, err = txDecl.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	txClose, err = txClose.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmPayment(CloseEnvelope{
		Details: ca,
		ProposerSignatures: CloseSignatures{
			Declaration: txDecl.Signatures()[0].Signature,
			Close:       txClose.Signatures()[0].Signature,
		},
	})
	assert.EqualError(t, err, "close agreement over commits: account is underfunded to make payment")
	assert.ErrorIs(t, err, ErrUnderfunded)

	// The same close payment should pass if the balance has been updated.
	initiatorChannel.UpdateRemoteMultiSigBalance(200)
	_, err = initiatorChannel.ConfirmPayment(CloseEnvelope{
		Details: ca,
		ProposerSignatures: CloseSignatures{
			Declaration: txDecl.Signatures()[0].Signature,
			Close:       txClose.Signatures()[0].Signature,
		},
	})
	assert.NoError(t, err)
}

func TestChannel_ConfirmPayment_responderRejectsPaymentThatIsUnderfunded(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   remoteSigner.Address(),
			ResponderSigner:   localSigner.Address(),
			InitiatorMultiSig: remoteMultiSigAccount.Address(),
			ResponderMultiSig: localMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = responderChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// A close agreement from the remote participant should be rejected if the
	// payment changes the balance in the favor of the remote.
	responderChannel.latestAuthorizedCloseAgreement = CloseAgreement{
		Envelope: CloseEnvelope{
			Details: CloseDetails{
				IterationNumber:            1,
				Balance:                    60, // Remote (initiator) owes local (responder) 60.
				ObservationPeriodTime:      10,
				ObservationPeriodLedgerGap: 10,
				ConfirmingSigner:           localSigner.FromAddress(),
			},
		},
	}

	ca := CloseDetails{
		IterationNumber:            2,
		Balance:                    110, // Remote (initiator) owes local (responder) 110, which initiator ❌ cannot pay.
		PaymentAmount:              50,
		ProposingSigner:            remoteSigner.FromAddress(),
		ConfirmingSigner:           localSigner.FromAddress(),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
	}
	txs, err := responderChannel.closeTxs(responderChannel.openAgreement.Envelope.Details, ca)
	txDecl := txs.Declaration
	txClose := txs.Close
	require.NoError(t, err)
	txDecl, err = txDecl.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	txClose, err = txClose.Sign(network.TestNetworkPassphrase, remoteSigner)
	require.NoError(t, err)
	_, err = responderChannel.ConfirmPayment(CloseEnvelope{
		Details: ca,
		ProposerSignatures: CloseSignatures{
			Declaration: txDecl.Signatures()[0].Signature,
			Close:       txClose.Signatures()[0].Signature,
		},
	})
	assert.EqualError(t, err, "close agreement over commits: account is underfunded to make payment")
	assert.ErrorIs(t, err, ErrUnderfunded)

	// The same close payment should pass if the balance has been updated.
	responderChannel.UpdateRemoteMultiSigBalance(200)
	_, err = responderChannel.ConfirmPayment(CloseEnvelope{
		Details: ca,
		ProposerSignatures: CloseSignatures{
			Declaration: txDecl.Signatures()[0].Signature,
			Close:       txClose.Signatures()[0].Signature,
		},
	})
	assert.NoError(t, err)
}

func TestChannel_ConfirmPayment_initiatorCannotProposePaymentThatIsUnderfunded(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   localSigner.Address(),
			ResponderSigner:   remoteSigner.Address(),
			InitiatorMultiSig: localMultiSigAccount.Address(),
			ResponderMultiSig: remoteMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// A close agreement from the remote participant should be rejected if the
	// payment changes the balance in the favor of the remote.
	initiatorChannel.latestAuthorizedCloseAgreement = CloseAgreement{
		Envelope: CloseEnvelope{
			Details: CloseDetails{
				IterationNumber:            1,
				Balance:                    60, // Local (initiator) owes remote (responder) 60.
				ObservationPeriodTime:      10,
				ObservationPeriodLedgerGap: 10,
				ConfirmingSigner:           localSigner.FromAddress(),
			},
		},
	}

	_, err := initiatorChannel.ProposePayment(110)
	assert.EqualError(t, err, "amount over commits: account is underfunded to make payment")
	assert.ErrorIs(t, err, ErrUnderfunded)

	// The same close payment should pass if the balance has been updated.
	initiatorChannel.UpdateLocalMultiSigBalance(200)
	_, err = initiatorChannel.ProposePayment(110)
	assert.NoError(t, err)
}

func TestChannel_ConfirmPayment_responderCannotProposePaymentThatIsUnderfunded(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:            NativeAsset,
			ExpiresAt:        time.Now().Add(5 * time.Minute),
			StartingSequence: 101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   remoteSigner.Address(),
			ResponderSigner:   localSigner.Address(),
			InitiatorMultiSig: remoteMultiSigAccount.Address(),
			ResponderMultiSig: localMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = responderChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// A close agreement from the remote participant should be rejected if the
	// payment changes the balance in the favor of the remote.
	responderChannel.latestAuthorizedCloseAgreement = CloseAgreement{
		Envelope: CloseEnvelope{
			Details: CloseDetails{
				IterationNumber:            1,
				Balance:                    -60, // Local (responder) owes remote (initiator) 60.
				ObservationPeriodTime:      10,
				ObservationPeriodLedgerGap: 10,
				ConfirmingSigner:           localSigner.FromAddress(),
			},
		},
	}

	_, err := responderChannel.ProposePayment(110)
	assert.EqualError(t, err, "amount over commits: account is underfunded to make payment")
	assert.ErrorIs(t, err, ErrUnderfunded)

	// The same close payment should pass if the balance has been updated.
	responderChannel.UpdateLocalMultiSigBalance(200)
	_, err = responderChannel.ProposePayment(110)
	assert.NoError(t, err)
}

func TestChannel_ProposeAndConfirmPayment_rejectNegativeAmountPayment(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			ObservationPeriodLedgerGap: 1,
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now().Add(5 * time.Minute),
			StartingSequence:           101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   remoteSigner.Address(),
			ResponderSigner:   localSigner.Address(),
			InitiatorMultiSig: remoteMultiSigAccount.Address(),
			ResponderMultiSig: localMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		err = responderChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err = responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	_, err := initiatorChannel.ProposePayment(-1)
	assert.EqualError(t, err, "payment amount must not be less than 0")

	// Propose a payment and modify it to be a valid payment with amount zero.
	ca, err := initiatorChannel.ProposePayment(0)
	require.NoError(t, err)
	ca.Envelope.Details.PaymentAmount = -1
	ca.Envelope.Details.Balance = -1
	txs, err := initiatorChannel.closeTxs(initiatorChannel.openAgreement.Envelope.Details, ca.Envelope.Details)
	require.NoError(t, err)
	sigs, err := signCloseAgreementTxs(txs, initiatorChannel.localSigner)
	require.NoError(t, err)
	ca.Envelope.ProposerSignatures = sigs
	_, err = responderChannel.ConfirmPayment(ca.Envelope)
	assert.EqualError(t, err, "close agreement is a payment to the proposer")
}

func TestChannel_ProposeAndConfirmPayment_allowZeroAmountPayment(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			ObservationPeriodLedgerGap: 1,
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now().Add(5 * time.Minute),
			StartingSequence:           101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   remoteSigner.Address(),
			ResponderSigner:   localSigner.Address(),
			InitiatorMultiSig: remoteMultiSigAccount.Address(),
			ResponderMultiSig: localMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		err = responderChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err = responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	ca, err := initiatorChannel.ProposePayment(0)
	require.NoError(t, err)

	_, err = responderChannel.ConfirmPayment(ca.Envelope)
	require.NoError(t, err)
}

func TestChannel_ProposeAndConfirmPayment_withMemo(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			ObservationPeriodLedgerGap: 1,
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now().Add(5 * time.Minute),
			StartingSequence:           101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   remoteSigner.Address(),
			ResponderSigner:   localSigner.Address(),
			InitiatorMultiSig: remoteMultiSigAccount.Address(),
			ResponderMultiSig: localMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		err = responderChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err = responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	initiatorChannel.UpdateLocalMultiSigBalance(100)
	responderChannel.UpdateRemoteMultiSigBalance(100)

	ca, err := initiatorChannel.ProposePaymentWithMemo(1, []byte("id1"))
	require.NoError(t, err)
	assert.Equal(t, []byte("id1"), ca.Envelope.Details.Memo)
	caResponse, err := responderChannel.ConfirmPayment(ca.Envelope)
	require.NoError(t, err)
	assert.Equal(t, []byte("id1"), caResponse.Envelope.Details.Memo)
	_, err = initiatorChannel.ConfirmPayment(caResponse.Envelope)
	require.NoError(t, err)
}

func TestChannel_ConfirmPayment_signatureChecks(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			ObservationPeriodLedgerGap: 10,
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now().Add(5 * time.Minute),
			StartingSequence:           101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   remoteSigner.Address(),
			ResponderSigner:   localSigner.Address(),
			InitiatorMultiSig: remoteMultiSigAccount.Address(),
			ResponderMultiSig: localMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = responderChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		cs, err := responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		err = initiatorChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		cs, err = initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}
	initiatorChannel.UpdateLocalMultiSigBalance(200)
	responderChannel.UpdateRemoteMultiSigBalance(200)

	ca, err := initiatorChannel.ProposePayment(100)
	require.NoError(t, err)

	// Pretend that the proposer did not sign any tx.
	caModified := ca
	caModified.Envelope.ProposerSignatures.Declaration = nil
	caModified.Envelope.ProposerSignatures.Close = nil
	_, err = responderChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")

	// Pretend that the proposer did not sign a tx.
	caModified = ca
	caModified.Envelope.ProposerSignatures.Declaration = nil
	_, err = responderChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")
	caModified = ca
	caModified.Envelope.ProposerSignatures.Close = nil
	_, err = responderChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")

	// Pretend that the proposer signed the txs invalidly.
	caModified = ca
	caModified.Envelope.ProposerSignatures.Declaration = caModified.Envelope.ProposerSignatures.Close
	_, err = responderChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")
	caModified = ca
	caModified.Envelope.ProposerSignatures.Close = caModified.Envelope.ProposerSignatures.Declaration
	_, err = responderChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")

	// Valid proposer signatures accepted by confirmer.
	ca, err = responderChannel.ConfirmPayment(ca.Envelope)
	require.NoError(t, err)

	// Pretend that no one signed.
	caModified = ca
	caModified.Envelope.ConfirmerSignatures.Declaration = nil
	caModified.Envelope.ConfirmerSignatures.Close = nil
	caModified.Envelope.ProposerSignatures.Declaration = nil
	caModified.Envelope.ProposerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")

	// Pretend that confirmer did not sign any tx.
	caModified = ca
	caModified.Envelope.ConfirmerSignatures.Declaration = nil
	caModified.Envelope.ConfirmerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")

	// Pretend that the confirmer did not sign a tx.
	caModified = ca
	caModified.Envelope.ConfirmerSignatures.Declaration = nil
	_, err = initiatorChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")
	caModified = ca
	caModified.Envelope.ConfirmerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")

	// Pretend that the confirmer signed the txs invalidly.
	caModified = ca
	caModified.Envelope.ConfirmerSignatures.Declaration = caModified.Envelope.ConfirmerSignatures.Close
	_, err = initiatorChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")
	caModified = ca
	caModified.Envelope.ConfirmerSignatures.Close = caModified.Envelope.ConfirmerSignatures.Declaration
	_, err = initiatorChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")

	// Pretend that proposer's signature is missing.
	caModified = ca
	caModified.Envelope.ProposerSignatures.Declaration = nil
	caModified.Envelope.ProposerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "not signed by local")

	// Pretend that the proposer's signature is missing for one tx.
	caModified = ca
	caModified.Envelope.ProposerSignatures.Declaration = nil
	_, err = initiatorChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")
	caModified = ca
	caModified.Envelope.ProposerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")

	// Pretend that the proposer's signature is invalid for one tx.
	caModified = ca
	caModified.Envelope.ProposerSignatures.Declaration = caModified.Envelope.ProposerSignatures.Close
	_, err = initiatorChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")
	caModified = ca
	caModified.Envelope.ProposerSignatures.Close = caModified.Envelope.ProposerSignatures.Declaration
	_, err = initiatorChannel.ConfirmPayment(caModified.Envelope)
	require.EqualError(t, err, "invalid signature: signature verification failed")

	// Valid proposer and confirmer signatures accepted by proposer.
	_, err = initiatorChannel.ConfirmPayment(ca.Envelope)
	require.NoError(t, err)
}

func TestChannel_FinalizePayment_noUnauthorizedAgreement(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			ObservationPeriodLedgerGap: 10,
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now().Add(5 * time.Minute),
			StartingSequence:           101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   remoteSigner.Address(),
			ResponderSigner:   localSigner.Address(),
			InitiatorMultiSig: remoteMultiSigAccount.Address(),
			ResponderMultiSig: localMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = responderChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		cs, err := responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		err = initiatorChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		cs, err = initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}
	initiatorChannel.UpdateLocalMultiSigBalance(200)
	responderChannel.UpdateRemoteMultiSigBalance(200)

	ca, err := initiatorChannel.ProposePayment(100)
	require.NoError(t, err)
	ca, err = responderChannel.ConfirmPayment(ca.Envelope)
	require.NoError(t, err)
	_, err = initiatorChannel.FinalizePayment(ca.Envelope.ConfirmerSignatures)
	require.NoError(t, err)

	// Try finalizing a payment when there is no unauthorized payment because
	// all payments have been authorized. Should error.
	_, err = initiatorChannel.FinalizePayment(ca.Envelope.ConfirmerSignatures)
	require.EqualError(t, err, "no unauthorized close agreement to finalize")
}

func TestChannel_FinalizePayment_signatureChecks(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})

	// Put channel into the Open state.
	{
		m, err := initiatorChannel.ProposeOpen(OpenParams{
			ObservationPeriodLedgerGap: 10,
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now().Add(5 * time.Minute),
			StartingSequence:           101,
		})
		require.NoError(t, err)
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   remoteSigner.Address(),
			ResponderSigner:   localSigner.Address(),
			InitiatorMultiSig: remoteMultiSigAccount.Address(),
			ResponderMultiSig: localMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = responderChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		cs, err := responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		err = initiatorChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		cs, err = initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}
	initiatorChannel.UpdateLocalMultiSigBalance(200)
	responderChannel.UpdateRemoteMultiSigBalance(200)

	ca, err := initiatorChannel.ProposePayment(100)
	require.NoError(t, err)
	ca, err = responderChannel.ConfirmPayment(ca.Envelope)
	require.NoError(t, err)

	// Pretend that confirmer did not sign any tx.
	_, err = initiatorChannel.FinalizePayment(CloseSignatures{})
	require.EqualError(t, err, "invalid signature: signature verification failed")

	// Pretend that the confirmer did not sign a tx.
	_, err = initiatorChannel.FinalizePayment(CloseSignatures{
		Declaration: ca.Envelope.ConfirmerSignatures.Declaration,
	})
	require.EqualError(t, err, "invalid signature: signature verification failed")
	_, err = initiatorChannel.FinalizePayment(CloseSignatures{
		Close: ca.Envelope.ConfirmerSignatures.Close,
	})
	require.EqualError(t, err, "invalid signature: signature verification failed")

	// Pretend that the confirmer signed the txs invalidly.
	_, err = initiatorChannel.FinalizePayment(CloseSignatures{
		Close: ca.Envelope.ConfirmerSignatures.Declaration,
	})
	require.EqualError(t, err, "invalid signature: signature verification failed")
	_, err = initiatorChannel.FinalizePayment(CloseSignatures{
		Declaration: ca.Envelope.ConfirmerSignatures.Close,
	})
	require.EqualError(t, err, "invalid signature: signature verification failed")

	// Valid proposer and confirmer signatures accepted by proposer.
	_, err = initiatorChannel.FinalizePayment(ca.Envelope.ConfirmerSignatures)
	require.NoError(t, err)
}

func TestLastConfirmedPayment(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()
	sendingChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	receiverChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
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
		m, err = receiverChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = sendingChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := sendingChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   localSigner.Address(),
			ResponderSigner:   remoteSigner.Address(),
			InitiatorMultiSig: localMultiSigAccount.Address(),
			ResponderMultiSig: remoteMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = sendingChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = receiverChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := sendingChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		cs, err = receiverChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	sendingChannel.UpdateLocalMultiSigBalance(1000)
	sendingChannel.UpdateRemoteMultiSigBalance(1000)
	receiverChannel.UpdateLocalMultiSigBalance(1000)
	receiverChannel.UpdateRemoteMultiSigBalance(1000)

	// Test the returned close agreemenets are as expected.
	ca, err := sendingChannel.ProposePayment(200)
	require.NoError(t, err)
	assert.Equal(t, ca, sendingChannel.latestUnauthorizedCloseAgreement)

	signedTxs := ca.SignedTransactions()
	assert.Equal(t, ca.Transactions.DeclarationHash, signedTxs.DeclarationHash)
	assert.Equal(t, ca.Transactions.CloseHash, signedTxs.CloseHash)
	assert.Len(t, signedTxs.Declaration.Signatures(), 1)
	assert.Len(t, signedTxs.Close.Signatures(), 1)

	caResponse, err := receiverChannel.ConfirmPayment(ca.Envelope)
	require.NoError(t, err)
	assert.Equal(t, caResponse, receiverChannel.latestAuthorizedCloseAgreement)

	signedTxs = caResponse.SignedTransactions()
	assert.Equal(t, caResponse.Transactions.DeclarationHash, signedTxs.DeclarationHash)
	assert.Equal(t, caResponse.Transactions.CloseHash, signedTxs.CloseHash)
	assert.Len(t, signedTxs.Declaration.Signatures(), 3)
	assert.Len(t, signedTxs.Close.Signatures(), 2)

	// Confirming a close agreement with same sequence number but different Amount should error
	caDifferent := CloseEnvelope{
		Details: CloseDetails{
			IterationNumber:            2,
			Balance:                    400,
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
		},
		ProposerSignatures:  ca.Envelope.ProposerSignatures,
		ConfirmerSignatures: ca.Envelope.ConfirmerSignatures,
	}
	_, err = sendingChannel.ConfirmPayment(caDifferent)
	require.EqualError(t, err, "validating payment: close agreement does not match the close agreement already in progress")
	assert.Equal(t, ca, sendingChannel.latestUnauthorizedCloseAgreement)

	// Confirming a payment with same sequence number and same amount should pass
	caFinal, err := sendingChannel.ConfirmPayment(caResponse.Envelope)
	require.NoError(t, err)
	assert.Equal(t, CloseAgreement{}, sendingChannel.latestUnauthorizedCloseAgreement)
	assert.Equal(t, caFinal, sendingChannel.latestAuthorizedCloseAgreement)
	assert.Equal(t, caFinal, caResponse)
}

func TestChannel_ProposeAndConfirmPayment_rejectIfChannelNotOpen(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	senderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		MaxOpenExpiry:         10 * time.Second,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
	})
	receiverChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		MaxOpenExpiry:         10 * time.Second,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
	})

	// Before open, proposing a payment should error.
	_, err := senderChannel.ProposePayment(10)
	require.EqualError(t, err, "cannot propose a payment before channel is opened")

	// Before open, confirming a payment should error.
	_, err = senderChannel.ConfirmPayment(CloseEnvelope{})
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
	m, err = receiverChannel.ConfirmOpen(m.Envelope)
	require.NoError(t, err)
	_, err = senderChannel.ConfirmOpen(m.Envelope)
	require.NoError(t, err)

	// Before an open is executed and validated, proposing and confirming a payment should error.
	assert.False(t, senderChannel.latestAuthorizedCloseAgreement.Envelope.Empty())
	_, err = senderChannel.ProposePayment(10)
	require.EqualError(t, err, "cannot propose a payment before channel is opened")

	_, err = senderChannel.ConfirmPayment(CloseEnvelope{})
	require.EqualError(t, err, "validating payment: cannot confirm a payment before channel is opened")

	// Put channel into the Open state.
	{
		ftx, err := senderChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   localSigner.Address(),
			ResponderSigner:   remoteSigner.Address(),
			InitiatorMultiSig: localMultiSigAccount.Address(),
			ResponderMultiSig: remoteMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = senderChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = receiverChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
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
	p := CloseEnvelope{
		Details: CloseDetails{
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
	ca, err = receiverChannel.ConfirmClose(ca.Envelope)
	require.NoError(t, err)
	_, err = senderChannel.ConfirmClose(ca.Envelope)
	require.NoError(t, err)

	// After a confirmed coordinated close, proposing a payment should error.
	_, err = senderChannel.ProposePayment(10)
	require.EqualError(t, err, "cannot propose payment after an accepted coordinated close")

	_, err = receiverChannel.ProposePayment(10)
	require.EqualError(t, err, "cannot propose payment after an accepted coordinated close")

	// After a confirmed coordinated close, confirming a payment should error.
	p = CloseEnvelope{
		Details: CloseDetails{
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
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	senderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		MaxOpenExpiry:         10 * time.Second,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
	})
	receiverChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		MaxOpenExpiry:         10 * time.Second,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
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
	m, err = receiverChannel.ConfirmOpen(m.Envelope)
	require.NoError(t, err)
	_, err = senderChannel.ConfirmOpen(m.Envelope)
	require.NoError(t, err)

	// Put channel into the Open state.
	{
		ftx, err := senderChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   localSigner.Address(),
			ResponderSigner:   remoteSigner.Address(),
			InitiatorMultiSig: localMultiSigAccount.Address(),
			ResponderMultiSig: remoteMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = senderChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = receiverChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := senderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		cs, err = receiverChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	senderChannel.UpdateLocalMultiSigBalance(1000)
	senderChannel.UpdateRemoteMultiSigBalance(1000)

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
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	// Given a channel with observation periods set to 1.
	initiatorChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
	})
	responderChannel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             false,
		LocalSigner:           remoteSigner,
		RemoteSigner:          localSigner.FromAddress(),
		LocalMultiSigAccount:  remoteMultiSigAccount,
		RemoteMultiSigAccount: localMultiSigAccount,
		MaxOpenExpiry:         2 * time.Hour,
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
		m, err = responderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = initiatorChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)

		ftx, err := initiatorChannel.OpenTx()
		require.NoError(t, err)
		ftxXDR, err := ftx.Base64()
		require.NoError(t, err)

		successResultXDR, err := txbuildtest.BuildResultXDR(true)
		require.NoError(t, err)
		resultMetaXDR, err := txbuildtest.BuildOpenResultMetaXDR(txbuildtest.OpenResultMetaParams{
			InitiatorSigner:   localSigner.Address(),
			ResponderSigner:   remoteSigner.Address(),
			InitiatorMultiSig: localMultiSigAccount.Address(),
			ResponderMultiSig: remoteMultiSigAccount.Address(),
			StartSequence:     101,
			Asset:             txnbuild.NativeAsset{},
		})
		require.NoError(t, err)

		err = initiatorChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = responderChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := initiatorChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		cs, err = responderChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}
	initiatorChannel.UpdateLocalMultiSigBalance(200)
	initiatorChannel.UpdateRemoteMultiSigBalance(200)

	responderChannel.UpdateLocalMultiSigBalance(200)
	responderChannel.UpdateRemoteMultiSigBalance(200)

	// Initiator proposes payment to Responder.
	ca, err := initiatorChannel.ProposePayment(50)
	require.NoError(t, err)
	assert.Equal(t, int64(50), ca.Envelope.Details.Balance)
	assert.Equal(t, int64(50), ca.Envelope.Details.PaymentAmount)

	// An incorrect PaymentAmount should error.
	ca.Envelope.Details.PaymentAmount = 49
	_, err = responderChannel.ConfirmPayment(ca.Envelope)
	require.EqualError(t, err, "validating payment: close agreement payment amount is unexpected: "+
		"current balance: 0 proposed balance: 50 payment amount: 49 initiator proposed: true")

	ca.Envelope.Details.PaymentAmount = 50
	ca, err = responderChannel.ConfirmPayment(ca.Envelope)
	require.NoError(t, err)
	ca, err = initiatorChannel.ConfirmPayment(ca.Envelope)
	require.NoError(t, err)

	// Responder proposes payment to Initiator.
	ca, err = responderChannel.ProposePayment(100)
	require.NoError(t, err)
	assert.Equal(t, int64(-50), ca.Envelope.Details.Balance)
	assert.Equal(t, int64(100), ca.Envelope.Details.PaymentAmount)

	// An incorrect Balance should error.
	ca.Envelope.Details.Balance = -49
	_, err = initiatorChannel.ConfirmPayment(ca.Envelope)
	require.EqualError(t, err, "validating payment: close agreement payment amount is unexpected: "+
		"current balance: 50 proposed balance: -49 payment amount: 100 initiator proposed: false")

	ca.Envelope.Details.Balance = -50
	ca, err = initiatorChannel.ConfirmPayment(ca.Envelope)
	require.NoError(t, err)
}
