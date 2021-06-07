package close

import (
	"github.com/stellar/experimental-payment-channels/sdk/txbuild"
	"github.com/stellar/go/txnbuild"
)

// TODO - does submitting happen in this method or outside of it? (probably in orchestration)
func (c *Channel) StartClose() (*txnbuild.Transaction, error) {
	// submit latest decl
	decl, err := txbuild.Declaration(txbuild.DeclarationParams{
		InitiatorEscrow:         c.initiatorEscrowAccount().Address,
		StartSequence:           c.startingSequence,
		IterationNumber:         c.latestCloseAgreement.IterationNumber,
		IterationNumberExecuted: 0,
	})
	if err != nil {
		return nil, err
	}

	decl, err = decl.AddSignatureDecorated(c.latestCloseAgreement.DeclarationSignatures...)
	if err != nil {
		return nil, err
	}

	return decl, nil
}

func (c *Channel) CloseCoordinated(id string) (newStatus string, err error) {
	// modify txClose to be able to submit immediately
	// sign
	// send to other
	return "", nil
}

func (c *Channel) CloseUncoordinated(id string) error {
	// submit latest txDecl
	// wait observation period O
	// subit latest txClose
	return nil
}
