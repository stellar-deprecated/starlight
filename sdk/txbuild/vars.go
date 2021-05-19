package txbuild

import (
	"time"
)

const (
	observationPeriodTime      = 1 * time.Minute
	averageLedgerDuration      = 5 * time.Second
	observationPeriodLedgerGap = int(observationPeriodTime / averageLedgerDuration)
)

const m = 2

func startSequenceOfIteration(startSequence int64, iterationNumber int64) int64 {
	return startSequence + iterationNumber*m
}

func int64ptr(i int64) *int64 {
	return &i
}
