package txbuildtest

import (
	"testing"

	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_txbuildtest_buildResultXDR(t *testing.T) {
	r, err := BuildResultXDR(true)
	require.NoError(t, err)

	var txResult xdr.TransactionResult
	err = xdr.SafeUnmarshalBase64(r, &txResult)
	require.NoError(t, err)
	assert.True(t, txResult.Successful())

	r, err = BuildResultXDR(false)
	require.NoError(t, err)

	err = xdr.SafeUnmarshalBase64(r, &txResult)
	require.NoError(t, err)
	assert.False(t, txResult.Successful())
}

func Test_txbuildtest_buildResultMetaXDR(t *testing.T) {
	led := []xdr.LedgerEntryData{
		{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: xdr.MustAddress("GAKDNXUGEIRGESAXOPUHU4GOWLVYGQFJVHQOGFXKBXDGZ7AKMPPSDDPV"),
			},
		},
		{
			Type: xdr.LedgerEntryTypeTrustline,
			TrustLine: &xdr.TrustLineEntry{
				AccountId: xdr.MustAddress("GAKDNXUGEIRGESAXOPUHU4GOWLVYGQFJVHQOGFXKBXDGZ7AKMPPSDDPV"),
				Balance:   xdr.Int64(100),
			},
		},
	}

	m, err := BuildResultMetaXDR(led)
	require.NoError(t, err)

	// Validate the ledger entry changes are correct.
	var txMeta xdr.TransactionMeta
	err = xdr.SafeUnmarshalBase64(m, &txMeta)
	require.NoError(t, err)

	txMetaV2, ok := txMeta.GetV2()
	require.True(t, ok)

	for _, o := range txMetaV2.Operations {
		for i, change := range o.Changes {
			updated, ok := change.GetUpdated()
			require.True(t, ok)
			assert.Equal(t, led[i], updated.Data)
		}
	}
}
