package txbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSequenceNumberToTransactionType(t *testing.T) {
	assert.Equal(t, TransactionTypeOpen, SequenceNumberToTransactionType(100, 100))
	assert.Equal(t, TransactionTypeOpen, SequenceNumberToTransactionType(101, 101))

	assert.Equal(t, TransactionTypeUnrecognized, SequenceNumberToTransactionType(100, 101))
	assert.Equal(t, TransactionTypeUnrecognized, SequenceNumberToTransactionType(101, 102))

	assert.Equal(t, TransactionTypeDeclaration, SequenceNumberToTransactionType(101, 103))
	assert.Equal(t, TransactionTypeClose, SequenceNumberToTransactionType(101, 104))

	assert.Equal(t, TransactionTypeDeclaration, SequenceNumberToTransactionType(100, 102))
	assert.Equal(t, TransactionTypeClose, SequenceNumberToTransactionType(100, 103))
}
