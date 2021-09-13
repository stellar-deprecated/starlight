package txbuild

const m = 2

func startSequenceOfIteration(startSequence int64, iterationNumber int64) int64 {
	return startSequence + iterationNumber*m
}

type TransactionType string

const (
	TransactionTypeDeclaration TransactionType = "declaration"
	TransactionTypeClose       TransactionType = "close"
)

func SequenceNumberToTransactionType(startingSeqNum, seqNum int64) TransactionType {
	if startingSeqNum%m == seqNum%m {
		return TransactionTypeDeclaration
	}
	return TransactionTypeClose
}
