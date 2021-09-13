package txbuild

const m = 2

func startSequenceOfIteration(startSequence int64, iterationNumber int64) int64 {
	return startSequence + iterationNumber*m
}

type TransactionType string

const (
	TransactionTypeFormation   TransactionType = "formation"
	TransactionTypeDeclaration TransactionType = "declaration"
	TransactionTypeClose       TransactionType = "close"
)

func SequenceNumberToTransactionType(startingSeqNum, seqNum int64) TransactionType {
	if startingSeqNum == seqNum {
		return TransactionTypeFormation
	} else if startingSeqNum+1 == seqNum {
		panic("invalid sequence number")
	} else if startingSeqNum%m == seqNum%m {
		return TransactionTypeDeclaration
	}
	return TransactionTypeClose
}
