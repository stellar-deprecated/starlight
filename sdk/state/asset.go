package state

import (
	"fmt"
	"strings"

	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
)

// Asset is a Stellar asset.
type Asset string

const NativeAsset = Asset("native")

// IsNative returns true if the asset is the native asset of the stellar
// network.
func (a Asset) IsNative() bool {
	return a.Asset().IsNative()
}

// Code returns the asset code.
func (a Asset) Code() string {
	return a.Asset().GetCode()
}

// Issuer returns the issuer of the asset.
func (a Asset) Issuer() string {
	return a.Asset().GetIssuer()
}

// Asset returns an asset from the stellar/go/txnbuild package with the
// same asset code and issuer, or a native asset if a native asset.
func (a Asset) Asset() txnbuild.Asset {
	parts := strings.SplitN(string(a), ":", 2)
	if len(parts) == 1 {
		return txnbuild.NativeAsset{}
	}
	return txnbuild.CreditAsset{
		Code:   parts[0],
		Issuer: parts[1],
	}
}

// StringCanonical returns a string friendly representation of the asset in
// canonical form.
func (a Asset) StringCanonical() string {
	if a.IsNative() {
		return xdr.AssetTypeToString[xdr.AssetTypeAssetTypeNative]
	}
	return fmt.Sprintf("%s:%s", a.Code(), a.Issuer())
}

// EqualTrustLineAsset returns true if the canonical strings of this asset and a
// given trustline asset are equal, else false.
func (a Asset) EqualTrustLineAsset(ta xdr.TrustLineAsset) bool {
	switch ta.Type {
	case xdr.AssetTypeAssetTypeNative, xdr.AssetTypeAssetTypeCreditAlphanum4, xdr.AssetTypeAssetTypeCreditAlphanum12:
		return ta.ToAsset().StringCanonical() == a.StringCanonical()
	}
	return false
}
