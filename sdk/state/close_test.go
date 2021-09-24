package state

import (
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

func TestChannel_CloseTx(t *testing.T) {
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
		},
	}
	ce := CloseEnvelope{
		Details: CloseDetails{
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 2,
			IterationNumber:            3,
			Balance:                    4,
			ProposingSigner:            localSigner.FromAddress(),
			ConfirmingSigner:           remoteSigner.FromAddress(),
		},
		ProposerSignatures: CloseSignatures{
			Declaration: xdr.Signature{0},
			Close:       xdr.Signature{1},
		},
		ConfirmerSignatures: CloseSignatures{
			Declaration: xdr.Signature{2},
			Close:       xdr.Signature{3},
		},
	}
	txs, err := channel.closeTxs(oe.Details, ce.Details)
	require.NoError(t, err)
	channel.openAgreement = OpenAgreement{Envelope: oe}
	channel.latestAuthorizedCloseAgreement = CloseAgreement{Envelope: ce, Transactions: txs}
	closeTxHash := txs.CloseHash

	// TODO: Compare the non-signature parts of the txs with the result of
	// channel.closeTxs() when there is an practical way of doing that added to
	// txnbuild.

	// Check signatures are populated.
	declTx, closeTx, err := channel.CloseTxs()
	require.NoError(t, err)
	assert.ElementsMatch(t, []xdr.DecoratedSignature{
		{Hint: localSigner.Hint(), Signature: []byte{0}},
		{Hint: remoteSigner.Hint(), Signature: []byte{2}},
		xdr.NewDecoratedSignatureForPayload([]byte{3}, remoteSigner.Hint(), closeTxHash[:]),
	}, declTx.Signatures())
	assert.ElementsMatch(t, []xdr.DecoratedSignature{
		{Hint: localSigner.Hint(), Signature: []byte{1}},
		{Hint: remoteSigner.Hint(), Signature: []byte{3}},
	}, closeTx.Signatures())

	// Check stored txs are used by replacing the stored tx with an identifiable
	// tx and checking that's what is used for the authorized closing transactions.
	testTx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{AccountID: localEscrowAccount.Address(), Sequence: 123456789},
		BaseFee:       txnbuild.MinBaseFee,
		Timebounds:    txnbuild.NewInfiniteTimeout(),
		Operations:    []txnbuild.Operation{&txnbuild.BumpSequence{}},
	})
	require.NoError(t, err)
	channel.latestAuthorizedCloseAgreement.Transactions = CloseTransactions{
		Declaration: testTx,
		Close:       testTx,
	}
	declTx, closeTx, err = channel.CloseTxs()
	require.NoError(t, err)
	assert.Equal(t, int64(123456789), declTx.SequenceNumber())
	assert.Equal(t, int64(123456789), closeTx.SequenceNumber())

	// Check stored txs are used by replacing the stored tx with an identifiable
	// tx and checking that's what is used when building the same tx as the
	// latest unauthorized tx.
	testTx, err = txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount: &txnbuild.SimpleAccount{AccountID: localEscrowAccount.Address(), Sequence: 987654321},
		BaseFee:       txnbuild.MinBaseFee,
		Timebounds:    txnbuild.NewInfiniteTimeout(),
		Operations:    []txnbuild.Operation{&txnbuild.BumpSequence{}},
	})
	require.NoError(t, err)
	channel.latestUnauthorizedCloseAgreement.Transactions = CloseTransactions{
		Declaration: testTx,
		Close:       testTx,
	}
	txs, err = channel.closeTxs(oe.Details, channel.latestUnauthorizedCloseAgreement.Envelope.Details)
	require.NoError(t, err)
	assert.Equal(t, int64(987654321), txs.Declaration.SequenceNumber())
	assert.Equal(t, int64(987654321), txs.Close.SequenceNumber())
}

func TestChannel_ProposeClose(t *testing.T) {
	localSigner := keypair.MustRandom()
	remoteSigner := keypair.MustRandom()
	localEscrowAccount := keypair.MustRandom().FromAddress()
	remoteEscrowAccount := keypair.MustRandom().FromAddress()

	localChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
		MaxOpenExpiry:       2 * time.Hour,
	})
	remoteChannel := NewChannel(Config{
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
		open1, err := localChannel.ProposeOpen(OpenParams{
			ObservationPeriodTime:      1,
			ObservationPeriodLedgerGap: 1,
			ExpiresAt:                  time.Now().Add(time.Hour),
			StartingSequence:           101,
		})
		require.NoError(t, err)
		open2, err := remoteChannel.ConfirmOpen(open1.Envelope)
		require.NoError(t, err)
		_, err = localChannel.ConfirmOpen(open2.Envelope)
		require.NoError(t, err)

		ftx, err := localChannel.OpenTx()
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

		err = localChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)
		err = remoteChannel.IngestTx(1, ftxXDR, successResultXDR, resultMetaXDR)
		require.NoError(t, err)

		cs, err := localChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)

		cs, err = remoteChannel.State()
		require.NoError(t, err)
		assert.Equal(t, StateOpen, cs)
	}

	// If the local proposes a close, the agreement will have them as the proposer.
	closeByLocal, err := localChannel.ProposeClose()
	require.NoError(t, err)
	assert.Equal(t, localSigner.FromAddress(), closeByLocal.Envelope.Details.ProposingSigner)
	assert.Equal(t, remoteSigner.FromAddress(), closeByLocal.Envelope.Details.ConfirmingSigner)

	// If the remote proposes a close, the agreement will have them as the proposer.
	closeByRemote, err := remoteChannel.ProposeClose()
	require.NoError(t, err)
	assert.Equal(t, remoteSigner.FromAddress(), closeByRemote.Envelope.Details.ProposingSigner)
	assert.Equal(t, localSigner.FromAddress(), closeByRemote.Envelope.Details.ConfirmingSigner)
}

func TestChannel_ProposeAndConfirmCoordinatedClose(t *testing.T) {
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
	{
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

	// Coordinated close.
	ca, err := senderChannel.ProposeClose()
	require.NoError(t, err)
	ca2, err := receiverChannel.ConfirmClose(ca.Envelope)
	require.NoError(t, err)
	_, err = senderChannel.ConfirmClose(ca2.Envelope)
	require.NoError(t, err)
}

func TestChannel_ProposeAndConfirmCoordinatedClose_rejectIfChannelNotOpen(t *testing.T) {
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

	// Before open, proposing a coordinated close should error.
	_, err := senderChannel.ProposeClose()
	require.EqualError(t, err, "cannot propose a coordinated close before channel is opened")

	// Before open, confirming a coordinated close should error.
	_, err = senderChannel.ConfirmClose(CloseEnvelope{})
	require.EqualError(t, err, "validating close agreement: cannot confirm a coordinated close before channel is opened")

	// Open channel.
	{
		m, err := senderChannel.ProposeOpen(OpenParams{
			Asset:                      NativeAsset,
			ExpiresAt:                  time.Now().Add(5 * time.Second),
			ObservationPeriodTime:      10,
			ObservationPeriodLedgerGap: 10,
		})
		require.NoError(t, err)
		m, err = receiverChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
		_, err = senderChannel.ConfirmOpen(m.Envelope)
		require.NoError(t, err)
	}

	// Before an open is executed and validated, proposing and confirming a payment should error.
	assert.False(t, senderChannel.latestAuthorizedCloseAgreement.Envelope.isEmpty())
	_, err = senderChannel.ProposeClose()
	require.EqualError(t, err, "cannot propose a coordinated close before channel is opened")

	_, err = senderChannel.ConfirmClose(CloseEnvelope{})
	require.EqualError(t, err, "validating close agreement: cannot confirm a coordinated close before channel is opened")
}

func TestChannel_ConfirmClose_signatureChecks(t *testing.T) {
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
		resultMetaXDR, err := txbuildtest.BuildFormationResultMetaXDR(txbuildtest.FormationResultMetaParams{
			InitiatorSigner: remoteSigner.Address(),
			ResponderSigner: localSigner.Address(),
			InitiatorEscrow: remoteEscrowAccount.Address(),
			ResponderEscrow: localEscrowAccount.Address(),
			StartSequence:   101,
			Asset:           txnbuild.NativeAsset{},
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
	initiatorChannel.UpdateLocalEscrowAccountBalance(200)
	responderChannel.UpdateRemoteEscrowAccountBalance(200)

	ca, err := initiatorChannel.ProposeClose()
	require.NoError(t, err)

	// Pretend that the proposer did not sign any tx.
	caModified := ca
	caModified.Envelope.ProposerSignatures.Declaration = nil
	caModified.Envelope.ProposerSignatures.Close = nil
	_, err = responderChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")

	// Pretend that the proposer did not sign a tx.
	caModified = ca
	caModified.Envelope.ProposerSignatures.Declaration = nil
	_, err = responderChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")
	caModified = ca
	caModified.Envelope.ProposerSignatures.Close = nil
	_, err = responderChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying close signed: signature verification failed")

	// Pretend that the proposer signed the txs invalidly.
	caModified = ca
	caModified.Envelope.ProposerSignatures.Declaration = caModified.Envelope.ProposerSignatures.Close
	_, err = responderChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")
	caModified = ca
	caModified.Envelope.ProposerSignatures.Close = caModified.Envelope.ProposerSignatures.Declaration
	_, err = responderChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying close signed: signature verification failed")

	// Valid proposer signatures accepted by confirmer.
	ca, err = responderChannel.ConfirmClose(ca.Envelope)
	require.NoError(t, err)

	// Pretend that no one signed.
	caModified = ca
	caModified.Envelope.ConfirmerSignatures.Declaration = nil
	caModified.Envelope.ConfirmerSignatures.Close = nil
	caModified.Envelope.ProposerSignatures.Declaration = nil
	caModified.Envelope.ProposerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")

	// Pretend that confirmer did not sign any tx.
	caModified = ca
	caModified.Envelope.ConfirmerSignatures.Declaration = nil
	caModified.Envelope.ConfirmerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")

	// Pretend that the confirmer did not sign a tx.
	caModified = ca
	caModified.Envelope.ConfirmerSignatures.Declaration = nil
	_, err = initiatorChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")
	caModified = ca
	caModified.Envelope.ConfirmerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying close signed: signature verification failed")

	// Pretend that the confirmer signed the txs invalidly.
	caModified = ca
	caModified.Envelope.ConfirmerSignatures.Declaration = caModified.Envelope.ConfirmerSignatures.Close
	_, err = initiatorChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying declaration signed: signature verification failed")
	caModified = ca
	caModified.Envelope.ConfirmerSignatures.Close = caModified.Envelope.ConfirmerSignatures.Declaration
	_, err = initiatorChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by remote: verifying close signed: signature verification failed")

	// Pretend that proposer's signature is missing.
	caModified = ca
	caModified.Envelope.ProposerSignatures.Declaration = nil
	caModified.Envelope.ProposerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying declaration signed: signature verification failed")

	// Pretend that the proposer's signature is missing for one tx.
	caModified = ca
	caModified.Envelope.ProposerSignatures.Declaration = nil
	_, err = initiatorChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying declaration signed: signature verification failed")
	caModified = ca
	caModified.Envelope.ProposerSignatures.Close = nil
	_, err = initiatorChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying close signed: signature verification failed")

	// Pretend that the proposer's signature is invalid for one tx.
	caModified = ca
	caModified.Envelope.ProposerSignatures.Declaration = caModified.Envelope.ProposerSignatures.Close
	_, err = initiatorChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying declaration signed: signature verification failed")
	caModified = ca
	caModified.Envelope.ProposerSignatures.Close = caModified.Envelope.ProposerSignatures.Declaration
	_, err = initiatorChannel.ConfirmClose(caModified.Envelope)
	require.EqualError(t, err, "not signed by local: verifying close signed: signature verification failed")

	// Valid proposer and confirmer signatures accepted by proposer.
	_, err = initiatorChannel.ConfirmClose(ca.Envelope)
	require.NoError(t, err)
}
