package txbuild

import "fmt"

const m = 2

func startSequenceOfIteration(startSequence int64, iterationNumber int64) int64 {
	return startSequence + iterationNumber*m
}

type TransactionType string

const (
	TransactionTypeUnrecognized TransactionType = "unrecognized"
	TransactionTypeOpen         TransactionType = "open"
	TransactionTypeDeclaration  TransactionType = "declaration"
	TransactionTypeClose        TransactionType = "close"
)

func SequenceNumberToTransactionType(startingSeqNum, seqNum int64) TransactionType {
	seqRelative := seqNum - startingSeqNum
	if seqRelative == 0 {
		return TransactionTypeOpen
	} else if seqRelative > 0 && seqRelative < m {
		return TransactionTypeUnrecognized
	} else if seqRelative%m == 0 {
		return TransactionTypeDeclaration
	} else if seqRelative%m == 1 {
		return TransactionTypeClose
	}
	panic(fmt.Errorf("unhandled sequence number: startingSeqNum=%d seqNum=%d", startingSeqNum, seqNum))
}
