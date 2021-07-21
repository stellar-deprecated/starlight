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
				},
			},
			OpenAgreement{
				Details: OpenAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
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
				},
				CloseSignatures: []xdr.DecoratedSignature{
					{
						Hint:      [4]byte{0, 1, 2, 3},
						Signature: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
					},
				},
			},
			OpenAgreement{
				Details: OpenAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
				},
				CloseSignatures: []xdr.DecoratedSignature{
					{
						Hint:      [4]byte{0, 1, 2, 3},
						Signature: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
					},
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
				},
				CloseSignatures: []xdr.DecoratedSignature{
					{
						Hint:      [4]byte{0, 1, 2, 3},
						Signature: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
					},
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
				},
				CloseSignatures: []xdr.DecoratedSignature{
					{
						Hint:      [4]byte{0, 1, 2, 3},
						Signature: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
					},
				},
			},
			OpenAgreement{
				Details: OpenAgreementDetails{
					ObservationPeriodTime:      time.Minute,
					ObservationPeriodLedgerGap: 2,
					Asset:                      "native",
					ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
				},
				CloseSignatures: []xdr.DecoratedSignature{
					{
						Hint:      [4]byte{0, 1, 2, 3},
						Signature: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9},
					},
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
		},
		FormationSignatures: []xdr.DecoratedSignature{{Hint: [4]byte{2, 2, 2, 2}, Signature: []byte{2}}},
	}

	formationTx, err := channel.OpenTx()
	require.NoError(t, err)
	// TODO: Compare the non-signature parts of formationTx with the result of
	// channel.openTx() when there is an practical way of doing that added to
	// txnbuild.
	assert.Equal(t, []xdr.DecoratedSignature{{Hint: [4]byte{2, 2, 2, 2}, Signature: []byte{2}}}, formationTx.Signatures())
}

func TestConfirmOpen_checkForExtraSignatures(t *testing.T) {
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
	senderChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           true,
		LocalSigner:         localSigner,
		RemoteSigner:        remoteSigner.FromAddress(),
		LocalEscrowAccount:  localEscrowAccount,
		RemoteEscrowAccount: remoteEscrowAccount,
	})
	receiverChannel := NewChannel(Config{
		NetworkPassphrase:   network.TestNetworkPassphrase,
		Initiator:           false,
		LocalSigner:         remoteSigner,
		RemoteSigner:        localSigner.FromAddress(),
		LocalEscrowAccount:  remoteEscrowAccount,
		RemoteEscrowAccount: localEscrowAccount,
	})

	p := OpenParams{
		ObservationPeriodTime:      time.Minute,
		ObservationPeriodLedgerGap: 2,
		Asset:                      "native",
		ExpiresAt:                  time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC),
	}

	m, err := senderChannel.ProposeOpen(p)
	require.NoError(t, err)

	m.CloseSignatures = append(m.CloseSignatures, xdr.DecoratedSignature{Signature: randomByteArray(t, 10)})

	// Extra signature should cause error when receiver confirms
	_, err = receiverChannel.ConfirmOpen(m)
	assert.Equal(t, OpenAgreement{}, receiverChannel.openAgreement)
	require.EqualError(t, err, "input open agreement has too many signatures, has declaration: 2, close: 3, formation: 2, max of 2 allowed for each")

	// Remove extra signature, now should succeed
	m.CloseSignatures = m.CloseSignatures[0:1]
	m, err = receiverChannel.ConfirmOpen(m)
	require.NoError(t, err)

	// Adding extra signature should fail when sender confirms
	m.DeclarationSignatures = append(m.DeclarationSignatures, xdr.DecoratedSignature{Signature: randomByteArray(t, 10)})
	_, err = senderChannel.ConfirmOpen(m)
	require.EqualError(t, err, "input open agreement has too many signatures, has declaration: 3, close: 2, formation: 2, max of 2 allowed for each")

	// Remove extra signature, now should succeed
	m.DeclarationSignatures = m.DeclarationSignatures[0:2]
	m, err = senderChannel.ConfirmOpen(m)
	require.NoError(t, err)

	// Extra signature should cause error when receiver confirms last time
	m.FormationSignatures = append(m.FormationSignatures, xdr.DecoratedSignature{Signature: randomByteArray(t, 10)})
	_, err = receiverChannel.ConfirmOpen(m)
	require.EqualError(t, err, "input open agreement has too many signatures, has declaration: 2, close: 2, formation: 3, max of 2 allowed for each")

	// Remove extra signature, now should succeed
	m.FormationSignatures = m.FormationSignatures[0:2]
	m, err = receiverChannel.ConfirmOpen(m)
	require.NoError(t, err)
}
