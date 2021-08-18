package txbuild

const m = 2

func startSequenceOfIteration(startSequence int64, iterationNumber int64) int64 {
	return startSequence + iterationNumber*m
}
