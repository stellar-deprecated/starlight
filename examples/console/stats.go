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
	paymentsSent             int64
	paymentsReceived         int64
	bufferedPaymentsSent     int64
	bufferedPaymentsReceived int64
}

func (s *stats) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timeStart = time.Time{}
	s.timeFinish = time.Time{}
	s.paymentsSent = 0
	s.paymentsReceived = 0
	s.bufferedPaymentsSent = 0
	s.bufferedPaymentsReceived = 0
}

func (s *stats) Clone() *stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &stats{
		paymentsSent:             s.paymentsSent,
		paymentsReceived:         s.paymentsReceived,
		bufferedPaymentsSent:     s.bufferedPaymentsSent,
		bufferedPaymentsReceived: s.bufferedPaymentsReceived,
	}
}

func (s *stats) Merge(o *stats) *stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &stats{
		paymentsSent:             s.paymentsSent + o.paymentsSent,
		paymentsReceived:         s.paymentsReceived + o.paymentsReceived,
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

func (s *stats) AddPaymentsSent(delta int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paymentsSent += int64(delta)
}

func (s *stats) AddPaymentsReceived(delta int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paymentsReceived += int64(delta)
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

func (s *stats) PaymentsPerSecond() float64 {
	timeFinish := s.timeFinish
	if timeFinish.IsZero() {
		timeFinish = time.Now()
	}
	duration := s.timeFinish.Sub(s.timeStart)
	pps := float64(s.paymentsSent+s.paymentsReceived) / duration.Seconds()
	if math.IsNaN(pps) || math.IsInf(pps, 0) {
		pps = 0
	}
	return pps
}

func (s *stats) BufferedPaymentsPerSecond() float64 {
	timeFinish := s.timeFinish
	if timeFinish.IsZero() {
		timeFinish = time.Now()
	}
	duration := timeFinish.Sub(s.timeStart)
	bpps := float64(s.bufferedPaymentsSent+s.bufferedPaymentsReceived) / duration.Seconds()
	if math.IsNaN(bpps) || math.IsInf(bpps, 0) {
		bpps = 0
	}
	return bpps
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
	fmt.Fprintf(&sb, "payments sent: %d\n", s.paymentsSent)
	fmt.Fprintf(&sb, "payments received: %d\n", s.paymentsReceived)
	fmt.Fprintf(&sb, "payments tps: %.3f\n", s.PaymentsPerSecond())
	fmt.Fprintf(&sb, "buffered payments sent: %d\n", s.bufferedPaymentsSent)
	fmt.Fprintf(&sb, "buffered payments received: %d\n", s.bufferedPaymentsReceived)
	fmt.Fprintf(&sb, "buffered payments tps: %.3f\n", s.BufferedPaymentsPerSecond())
	return sb.String()
}

func (s *stats) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := struct {
		PaymentsSent              int64
		PaymentsReceived          int64
		PaymentsPerSecond         int64
		BufferedPaymentsSent      int64
		BufferedPaymentsReceived  int64
		BufferedPaymentsPerSecond int64
	}{
		PaymentsSent:              s.paymentsSent,
		PaymentsReceived:          s.paymentsReceived,
		PaymentsPerSecond:         int64(s.PaymentsPerSecond()),
		BufferedPaymentsSent:      s.bufferedPaymentsSent,
		BufferedPaymentsReceived:  s.bufferedPaymentsReceived,
		BufferedPaymentsPerSecond: int64(s.BufferedPaymentsPerSecond()),
	}
	return json.MarshalIndent(v, "", "  ")
}
