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

func TestChannel_ConfirmOpen_signatureChecks(t *testing.T) {
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

	oa, err := initiatorChannel.ProposeOpen(OpenParams{
		ObservationPeriodLedgerGap: 10,
		Asset:                      NativeAsset,
		ExpiresAt:                  time.Now().Add(5 * time.Minute),
		StartingSequence:           101,
	})
	require.NoError(t, err)

	// Pretend that the proposer did not sign any tx.
	oaModified := oa
	oaModified.Envelope.ProposerSignatures.Formation = nil
	oaModified.Envelope.ProposerSignatures.Declaration = nil
	oaModified.Envelope.ProposerSignatures.Close = nil
	_, err = responderChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")

	// Pretend that the proposer did not sign a tx.
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Formation = nil
	_, err = responderChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying formation signed: signature verification failed")
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
	oaModified.Envelope.ProposerSignatures.Formation = oaModified.Envelope.ProposerSignatures.Close
	_, err = responderChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying formation signed: signature verification failed")
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
	oaModified.Envelope.ConfirmerSignatures.Formation = nil
	oaModified.Envelope.ConfirmerSignatures.Declaration = nil
	oaModified.Envelope.ConfirmerSignatures.Close = nil
	oaModified.Envelope.ProposerSignatures.Formation = nil
	oaModified.Envelope.ProposerSignatures.Declaration = nil
	oaModified.Envelope.ProposerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")

	// Pretend that confirmer did not sign any tx.
	oaModified = oa
	oaModified.Envelope.ConfirmerSignatures.Formation = nil
	oaModified.Envelope.ConfirmerSignatures.Declaration = nil
	oaModified.Envelope.ConfirmerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")

	// Pretend that the confirmer did not sign a tx.
	oaModified = oa
	oaModified.Envelope.ConfirmerSignatures.Formation = nil
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying formation signed: signature verification failed")
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
	oaModified.Envelope.ConfirmerSignatures.Formation = oaModified.Envelope.ConfirmerSignatures.Close
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying formation signed: signature verification failed")
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
	oaModified.Envelope.ProposerSignatures.Formation = nil
	oaModified.Envelope.ProposerSignatures.Declaration = nil
	oaModified.Envelope.ProposerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying declaration signed: signature verification failed")

	// Pretend that the proposer's signature is missing for one tx.
	oaModified = oa
	oaModified.Envelope.ProposerSignatures.Formation = nil
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying formation signed: signature verification failed")
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
	oaModified.Envelope.ProposerSignatures.Formation = oaModified.Envelope.ProposerSignatures.Close
	_, err = initiatorChannel.ConfirmOpen(oaModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying formation signed: signature verification failed")
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
