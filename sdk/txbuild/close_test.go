package txbuild

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClose_iterationNumber_checkNonNegative(t *testing.T) {
	_, err := Close(CloseParams{
		StartSequence:   101,
		IterationNumber: -1,
	})
	assert.EqualError(t, err, "invalid iteration number or start sequence: cannot be negative")
	_, err = Close(CloseParams{
		StartSequence:   -1,
		IterationNumber: 5,
	})
	assert.EqualError(t, err, "invalid iteration number or start sequence: cannot be negative")
}

func TestClose_startSequenceOfIteration_checkNonNegative(t *testing.T) {
	_, err := Close(CloseParams{
		IterationNumber: 0,
		StartSequence:   math.MaxInt64,
	})
	assert.EqualError(t, err, "invalid sequence number: cannot be negative")
}
