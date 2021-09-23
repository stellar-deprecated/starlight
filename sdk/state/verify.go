package state

import (
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/xdr"
	"golang.org/x/sync/errgroup"
)

type signatureVerificationInput struct {
	Payload   []byte
	Signature xdr.Signature
	Signer    *keypair.FromAddress
}

func verifySignatures(inputs []signatureVerificationInput) error {
	g := errgroup.Group{}
	for _, i := range inputs {
		i := i
		g.Go(func() error {
			return i.Signer.Verify(i.Payload, []byte(i.Signature))
		})
	}
	return g.Wait()
}
