package state

import (
	"math"
	"strconv"
	"testing"
	"time"

	fuzz "github.com/google/gofuzz"
	"github.com/stellar/experimental-payment-channels/sdk/txbuildtest"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenDetails_Equal(t *testing.T) {
	assert.True(t, OpenDetails{}.Equal(OpenDetails{}))

	// The same value should be equal.
	ft := time.Now().UnixNano()
	od1 := OpenDetails{}
	fuzz.NewWithSeed(ft).NilChance(0).Fuzz(&od1)
	t.Log("od1:", od1)
	od2 := OpenDetails{}
	fuzz.NewWithSeed(ft).NilChance(0).Fuzz(&od2)
	t.Log("od2:", od2)
	assert.True(t, od1.Equal(od2))

	// Different values should never be equal.
	for i := 0; i < 20; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			f := fuzz.New()
			a := OpenSignatures{}
			f.Fuzz(&a)
			t.Log("a:", a)
			b := OpenSignatures{}
			f.Fuzz(&b)
			t.Log("b:", b)
			assert.False(t, a.Equal(b))
			assert.False(t, b.Equal(a))
		})
	}
}

func TestOpenSignatures_Equal(t *testing.T) {
	assert.True(t, OpenSignatures{}.Equal(OpenSignatures{}))

	// The same value should be equal. It is common for OpenSignatures to be
	// defined in whole or not at all, so we test that use case.
	ft := time.Now().UnixNano()
	os1 := OpenSignatures{}
	fuzz.NewWithSeed(ft).NilChance(0).Fuzz(&os1)
	t.Log("os1:", os1)
	os2 := OpenSignatures{}
	fuzz.NewWithSeed(ft).NilChance(0).Fuzz(&os2)
	t.Log("os2:", os2)
	assert.True(t, os1.Equal(os2))

	// Different values should never be equal.
	for i := 0; i < 20; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			f := fuzz.New()
			a := OpenSignatures{}
			f.Fuzz(&a)
			t.Log("a:", a)
			b := OpenSignatures{}
			f.Fuzz(&b)
			t.Log("b:", b)
			assert.False(t, a.Equal(b))
			assert.False(t, b.Equal(a))
		})
	}
}

func TestOpenEnvelope_Equal(t *testing.T) {
	assert.True(t, OpenEnvelope{}.Equal(OpenEnvelope{}))

	// The same value should be equal. It's common for OpenEnvelopes to start
	// with details then have signatures added, so we check that pattern of
	// incrementally adding fields.
	f := fuzz.New().NilChance(0)
	od := OpenDetails{}
	f.Fuzz(&od)
	t.Log("od:", od)
	ps := OpenSignatures{}
	f.Fuzz(&ps)
	t.Log("ps:", ps)
	cs := OpenSignatures{}
	f.Fuzz(&cs)
	t.Log("cs:", cs)
	assert.True(t, OpenEnvelope{Details: od}.Equal(OpenEnvelope{Details: od}))
	assert.True(t, OpenEnvelope{Details: od, ProposerSignatures: ps}.Equal(OpenEnvelope{Details: od, ProposerSignatures: ps}))
	assert.True(t, OpenEnvelope{Details: od, ProposerSignatures: ps, ConfirmerSignatures: cs}.Equal(OpenEnvelope{Details: od, ProposerSignatures: ps, ConfirmerSignatures: cs}))

	// Different values should never be equal.
	for i := 0; i < 20; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			f := fuzz.New()
			a := OpenEnvelope{}
			f.Fuzz(&a)
			t.Log("a:", a)
			b := OpenEnvelope{}
			f.Fuzz(&b)
			t.Log("b:", b)
			assert.False(t, a.Equal(b))
			assert.False(t, b.Equal(a))
		})
	}
}

func TestProposeOpen_validAsset(t *testing.T) {
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
	})
	_, err := sendingChannel.ProposeOpen(OpenParams{
		Asset:            NativeAsset,
		ExpiresAt:        time.Now().Add(5 * time.Minute),
		StartingSequence: 1,
	})
	require.NoError(t, err)

	sendingChannel = NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
	})
	_, err = sendingChannel.ProposeOpen(OpenParams{
		Asset:            "ABCD:GCSZIQEYTDI427C2XCCIWAGVHOIZVV2XKMRELUTUVKOODNZWSR2OLF6P",
		ExpiresAt:        time.Now().Add(5 * time.Minute),
		StartingSequence: 1,
	})
	require.NoError(t, err)
}

func TestConfirmOpen_rejectsDifferentOpenAgreements(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	channel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
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

func TestConfirmOpen_rejectsOpenAgreementsWithLongOpen(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	channel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		MaxOpenExpiry:         10 * time.Second,
		Initiator:             true,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
	})

	_, err := channel.ConfirmOpen(OpenEnvelope{Details: OpenDetails{
		ObservationPeriodTime:      1,
		ObservationPeriodLedgerGap: 1,
		Asset:                      NativeAsset,
		ExpiresAt:                  time.Now().Add(100 * time.Second),
	}})
	require.EqualError(t, err, "validating open agreement: input open agreement expire too far into the future")
}

func TestChannel_ConfirmOpen_signatureChecks(t *testing.T) {
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

	oa, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodLedgerGap: 10,
		Asset:                      NativeAsset,
		ExpiresAt:                  time.Now().Add(5 * time.Minute),
		StartingSequence:           101,
	})
	require.NoError(t, err)

	// Pretend that the proposer did not sign any tx.
	oaModified := oa
	oaModified.Envelope.ProposerSignatures.Open = nil
	oaModified.Envelope.ProposerSignatures.Declaration = nil
	oaModified.Envelope.ProposerSignatures.Close = nil
	_, err = responderChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")

	// Pretend that the proposer did not sign a tx.
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Open = nil
	_, err = responderChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying open signed: signature verification failed")
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Declaration = nil
	_, err = responderChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Close = nil
	_, err = responderChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying close signed: signature verification failed")

	// Pretend that the proposer signed the txs invalidly.
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Open = oaModified.Envelope.ProposerSignatures.Close
	_, err = responderChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying open signed: signature verification failed")
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Declaration = oaModified.Envelope.ProposerSignatures.Close
	_, err = responderChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Close = oaModified.Envelope.ProposerSignatures.Declaration
	_, err = responderChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying close signed: signature verification failed")

	// Valid proposer signatures accepted by confirmer.
	oa, err = responderChannel.ConfirmOpen(oa.Envelope)
	require.NoError(t, err)

	// Pretend that no one signed.
	oaModified = oa
	oaModified.Envelope.ConfirmerSignatures.Open = nil
	oaModified.Envelope.ConfirmerSignatures.Declaration = nil
	oaModified.Envelope.ConfirmerSignatures.Close = nil
	oaModified.Envelope.ProposerSignatures.Open = nil
	oaModified.Envelope.ProposerSignatures.Declaration = nil
	oaModified.Envelope.ProposerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")

	// Pretend that confirmer did not sign any tx.
	oaModified = oa
	oaModified.Envelope.ConfirmerSignatures.Open = nil
	oaModified.Envelope.ConfirmerSignatures.Declaration = nil
	oaModified.Envelope.ConfirmerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")

	// Pretend that the confirmer did not sign a tx.
	oaModified = oa
	oaModified.Envelope.ConfirmerSignatures.Open = nil
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying open signed: signature verification failed")
	oaModified = oa
	oaModified.Envelope.ConfirmerSignatures.Declaration = nil
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")
	oaModified = oa
	oaModified.Envelope.ConfirmerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying close signed: signature verification failed")

	// Pretend that the confirmer signed the txs invalidly.
	oaModified = oa
	oaModified.Envelope.ConfirmerSignatures.Open = oaModified.Envelope.ConfirmerSignatures.Close
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying open signed: signature verification failed")
	oaModified = oa
	oaModified.Envelope.ConfirmerSignatures.Declaration = oaModified.Envelope.ConfirmerSignatures.Close
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")
	oaModified = oa
	oaModified.Envelope.ConfirmerSignatures.Close = oaModified.Envelope.ConfirmerSignatures.Declaration
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying close signed: signature verification failed")

	// Pretend that proposer's signature is missing.
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Open = nil
	oaModified.Envelope.ProposerSignatures.Declaration = nil
	oaModified.Envelope.ProposerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying declaration signed: signature verification failed")

	// Pretend that the proposer's signature is missing for one tx.
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Open = nil
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying open signed: signature verification failed")
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Declaration = nil
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying declaration signed: signature verification failed")
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying close signed: signature verification failed")

	// Pretend that the proposer's signature is invalid for one tx.
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Open = oaModified.Envelope.ProposerSignatures.Close
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying open signed: signature verification failed")
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Declaration = oaModified.Envelope.ProposerSignatures.Close
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying declaration signed: signature verification failed")
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Close = oaModified.Envelope.ProposerSignatures.Declaration
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying close signed: signature verification failed")

	// Valid proposer and confirmer signatures accepted by proposer.
	_, err = initiatorChannel.ConfirmOpen(oa.Envelope)
	require.NoError(t, err)
}

func TestChannel_OpenTx(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

	channel := NewChannel(Config{
		NetworkPassphrase:     network.TestNetworkPassphrase,
		Initiator:             true,
		LocalSigner:           localSigner,
		RemoteSigner:          remoteSigner.FromAddress(),
		LocalMultiSigAccount:  localMultiSigAccount,
		RemoteMultiSigAccount: remoteMultiSigAccount,
	})
	oe := OpenEnvelope{
		Details: OpenDetails{
			StartingSequence:           1,
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
			Open:        xdr.Signature{2},
		},
		ConfirmerSignatures: OpenSignatures{
			Declaration: xdr.Signature{3},
			Close:       xdr.Signature{4},
			Open:        xdr.Signature{5},
		},
	}
	txs, closeTxs, err := channel.openTxs(oe.Details)
	require.NoError(t, err)
	channel.openAgreement = OpenAgreement{Envelope: oe, Transactions: txs, CloseTransactions: closeTxs}
	declTxHash := closeTxs.DeclarationHash
	closeTxHash := closeTxs.CloseHash

	// TODO: Compare the non-signature parts of openTx with the result of
	// channel.openTx() when there is an practical way of doing that added to
	// txnbuild.

	// Check signatures are populated.
	openTx, err := channel.OpenTx()
	require.NoError(t, err)
	assert.ElementsMatch(t, []xdr.DecoratedSignature{
		{Hint: localSigner.Hint(), Signature: []byte{2}},
		{Hint: remoteSigner.Hint(), Signature: []byte{5}},
		xdr.NewDecoratedSignatureForPayload([]byte{3}, remoteSigner.Hint(), declTxHash[:]),
		xdr.NewDecoratedSignatureForPayload([]byte{4}, remoteSigner.Hint(), closeTxHash[:]),
	}, openTx.Signatures())

	// Check stored txs are used by replacing the stored tx with an identifiable
	// tx and checking that's what is used.
	testTx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{AccountID: localMultiSigAccount.Address(), Sequence: 123456789},
		BaseFee:       txnbuild.MinBaseFee,
		Timebounds:    txnbuild.NewInfiniteTimeout(),
		Operations:    []txnbuild.Operation{&txnbuild.BumpSequence{}},
	})
	require.NoError(t, err)
	channel.openAgreement.Transactions = OpenTransactions{
		Open: testTx,
	}
	openTx, err = channel.OpenTx()
	require.NoError(t, err)
	assert.Equal(t, int64(123456789), openTx.SequenceNumber())
}

func TestChannel_ProposeAndConfirmOpen_rejectIfChannelAlreadyOpeningOrAlreadyOpened(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

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

	// Open channel.
	m, err := initiatorChannel.ProposeOpen(OpenParams{
		Asset:                      NativeAsset,
		ExpiresAt:                  time.Now().Add(5 * time.Second),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
		StartingSequence:           101,
	})
	require.NoError(t, err)

	// Try proposing a second open.
	_, err = initiatorChannel.ProposeOpen(OpenParams{
		Asset:                      NativeAsset,
		ExpiresAt:                  time.Now().Add(5 * time.Second),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
	})
	require.EqualError(t, err, "cannot propose a new open if channel is already opening or already open")

	// Continue with the first open to successfully open.
	m, err = responderChannel.ConfirmOpen(m.Envelope)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(m.Envelope)
	require.NoError(t, err)

	{
		// Ingest the openTx successfully to enter the Open state.
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

	_, err = initiatorChannel.ProposeOpen(OpenParams{
		Asset:                      NativeAsset,
		ExpiresAt:                  time.Now().Add(5 * time.Second),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
	})
	require.EqualError(t, err, "cannot propose a new open if channel is already opening or already open")

	_, err = responderChannel.ConfirmOpen(m.Envelope)
	require.EqualError(t, err, "validating open agreement: cannot confirm a new open if channel is already opened")
}

func TestChannel_OpenAndPayment_sequenceOverflow(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localMultiSigAccount := keypair.MustRandom().FromAddress()
	remoteMultiSigAccount := keypair.MustRandom().FromAddress()

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

	// Proposing or Confirming Open that would move s over maxint64 should error.
	for i := 0; i < 3; i++ {
		_, err := initiatorChannel.ProposeOpen(OpenParams{
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now().Add(5 * time.Second),
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
			StartingSequence:           math.MaxInt64 - int64(i),
		})
		assert.EqualError(t, err, "building close txs for open: invalid sequence number: cannot be negative")

		_, err = initiatorChannel.ConfirmOpen(OpenEnvelope{Details: OpenDetails{
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 1,
			Asset:                      NativeAsset,
			StartingSequence:           math.MaxInt64 - int64(i),
		}})
		assert.EqualError(t, err, "building close txs for open: invalid sequence number: cannot be negative")
	}

	// Successful Open with the max start sequence allowed.
	m, err := initiatorChannel.ProposeOpen(OpenParams{
		Asset:                      NativeAsset,
		ExpiresAt:                  time.Now().Add(5 * time.Second),
		ObservationPeriodTime:      10,
		ObservationPeriodLedgerGap: 10,
		StartingSequence:           math.MaxInt64 - 3,
	})
	require.NoError(t, err)
	m, err = responderChannel.ConfirmOpen(m.Envelope)
	require.NoError(t, err)
	_, err = initiatorChannel.ConfirmOpen(m.Envelope)
	require.NoError(t, err)

	{
		// Ingest the openTx successfully to enter the Open state.
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
			StartSequence:     math.MaxInt64 - 3,
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

	// Proposing Payment that pushes the sequence number over max int64 should error.
	_, err = initiatorChannel.ProposePayment(10)
	assert.EqualError(t, err, "invalid sequence number: cannot be negative")
	initiatorChannel.latestUnauthorizedCloseAgreement = CloseAgreement{}

	// Confirming Payment that pushes the sequence number over max int64 should error.
	_, err = initiatorChannel.ConfirmPayment(CloseEnvelope{
		Details: CloseDetails{
			IterationNumber:            2,
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
			Balance:                    0,
			ConfirmingSigner:           localSigner.FromAddress(),
			ProposingSigner:            remoteSigner.FromAddress(),
		},
	})
	assert.EqualError(t, err, "invalid sequence number: cannot be negative")
}
