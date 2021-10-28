package txbuild

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeclaration_iterationNumber_checkNonNegative(t *testing.T) {
	_, err := Declaration(DeclarationParams{
		StartSequence:   101,
		IterationNumber: -1,
	})
	assert.EqualError(t, err, "invalid iteration number or start sequence: cannot be negative")
	_, err = Declaration(DeclarationParams{
		StartSequence:   -1,
		IterationNumber: 5,
	})
	assert.EqualError(t, err, "invalid iteration number or start sequence: cannot be negative")
}

func TestDeclaration_startSequenceOfIteration_checkNonNegative(t *testing.T) {
	_, err := Declaration(DeclarationParams{
		IterationNumber: 1,
		StartSequence:   math.MaxInt64,
	})
	assert.EqualError(t, err, "invalid sequence number: cannot be negative")
}
