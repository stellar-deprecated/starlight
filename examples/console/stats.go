package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

type stats struct {
	mu                       sync.RWMutex
	timeStart                time.Time
	timeFinish               time.Time
	agreementsSent           int64
	agreementsReceived       int64
	bufferedPaymentsSent     int64
	bufferedPaymentsReceived int64
}

func (s *stats) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timeStart = time.Time{}
	s.timeFinish = time.Time{}
	s.agreementsSent = 0
	s.agreementsReceived = 0
	s.bufferedPaymentsSent = 0
	s.bufferedPaymentsReceived = 0
}

func (s *stats) Clone() *stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &stats{
		agreementsSent:           s.agreementsSent,
		agreementsReceived:       s.agreementsReceived,
		bufferedPaymentsSent:     s.bufferedPaymentsSent,
		bufferedPaymentsReceived: s.bufferedPaymentsReceived,
	}
}

func (s *stats) Merge(o *stats) *stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &stats{
		agreementsSent:           s.agreementsSent + o.agreementsSent,
		agreementsReceived:       s.agreementsReceived + o.agreementsReceived,
		bufferedPaymentsSent:     s.bufferedPaymentsSent + o.bufferedPaymentsSent,
		bufferedPaymentsReceived: s.bufferedPaymentsReceived + o.bufferedPaymentsReceived,
	}
}

func (s *stats) MarkStart() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.timeStart.IsZero() {
		panic("marking start of stats when already marked")
	}
	s.timeStart = time.Now()
}

func (s *stats) MarkFinish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.timeFinish.IsZero() {
		panic("marking finish of stats when already marked")
	}
	s.timeFinish = time.Now()
}

func (s *stats) AddAgreementsSent(delta int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agreementsSent += int64(delta)
}

func (s *stats) AddAgreementsReceived(delta int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agreementsReceived += int64(delta)
}

func (s *stats) AddBufferedPaymentsSent(delta int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bufferedPaymentsSent += int64(delta)
}

func (s *stats) AddBufferedPaymentsReceived(delta int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bufferedPaymentsReceived += int64(delta)
}

func (s *stats) AgreementsPerSecond() float64 {
	timeFinish := s.timeFinish
	if timeFinish.IsZero() {
		timeFinish = time.Now()
	}
	duration := s.timeFinish.Sub(s.timeStart)
	rate := float64(s.agreementsSent+s.agreementsReceived) / duration.Seconds()
	if math.IsNaN(rate) || math.IsInf(rate, 0) {
		rate = 0
	}
	return rate
}

func (s *stats) BufferedPaymentsPerSecond() float64 {
	timeFinish := s.timeFinish
	if timeFinish.IsZero() {
		timeFinish = time.Now()
	}
	duration := timeFinish.Sub(s.timeStart)
	rate := float64(s.bufferedPaymentsSent+s.bufferedPaymentsReceived) / duration.Seconds()
	if math.IsNaN(rate) || math.IsInf(rate, 0) {
		rate = 0
	}
	return rate
}

func (s *stats) Summary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sb := strings.Builder{}
	timeFinish := s.timeFinish
	if timeFinish.IsZero() {
		timeFinish = time.Now()
	}
	duration := timeFinish.Sub(s.timeStart)
	fmt.Fprintf(&sb, "time spent: %v\n", duration)
	fmt.Fprintf(&sb, "agreements sent: %d\n", s.agreementsSent)
	fmt.Fprintf(&sb, "agreements received: %d\n", s.agreementsReceived)
	fmt.Fprintf(&sb, "agreements tps: %.3f\n", s.AgreementsPerSecond())
	fmt.Fprintf(&sb, "buffered payments sent: %d\n", s.bufferedPaymentsSent)
	fmt.Fprintf(&sb, "buffered payments received: %d\n", s.bufferedPaymentsReceived)
	fmt.Fprintf(&sb, "buffered payments tps: %.3f\n", s.BufferedPaymentsPerSecond())
	return sb.String()
}

func (s *stats) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := struct {
		AgreementsSent            int64
		AgreementsReceived        int64
		AgreementsPerSecond       int64
		BufferedPaymentsSent      int64
		BufferedPaymentsReceived  int64
		BufferedPaymentsPerSecond int64
	}{
		AgreementsSent:            s.agreementsSent,
		AgreementsReceived:        s.agreementsReceived,
		AgreementsPerSecond:       int64(s.AgreementsPerSecond()),
		BufferedPaymentsSent:      s.bufferedPaymentsSent,
		BufferedPaymentsReceived:  s.bufferedPaymentsReceived,
		BufferedPaymentsPerSecond: int64(s.BufferedPaymentsPerSecond()),
	}
	return json.MarshalIndent(v, "", "  ")
}
