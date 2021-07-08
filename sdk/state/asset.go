package state

import (
	"strings"

	"github.com/stellar/go/txnbuild"
)

type Asset string

const NativeAsset = Asset("native")

func (a Asset) IsNative() bool {
	return a.Asset().IsNative()
}

func (a Asset) Code() string {
	return a.Asset().GetCode()
}

func (a Asset) Issuer() string {
	return a.Asset().GetIssuer()
}

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
