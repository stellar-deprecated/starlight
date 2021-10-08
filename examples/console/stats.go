package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type stats struct {
	paymentsSent             int64
	paymentsReceived         int64
	bufferedPaymentsSent     int64
	bufferedPaymentsReceived int64
}

func (s *stats) Reset() {
	atomic.StoreInt64(&s.paymentsSent, 0)
	atomic.StoreInt64(&s.paymentsReceived, 0)
	atomic.StoreInt64(&s.bufferedPaymentsSent, 0)
	atomic.StoreInt64(&s.bufferedPaymentsReceived, 0)
}

func (s *stats) AddPaymentsSent(delta int) {
	atomic.AddInt64(&s.paymentsSent, int64(delta))
}

func (s *stats) AddPaymentsReceived(delta int) {
	atomic.AddInt64(&s.paymentsReceived, int64(delta))
}

func (s *stats) AddBufferedPaymentsSent(delta int) {
	atomic.AddInt64(&s.bufferedPaymentsSent, int64(delta))
}

func (s *stats) AddBufferedPaymentsReceived(delta int) {
	atomic.AddInt64(&s.bufferedPaymentsReceived, int64(delta))
}

func (s stats) GetSummary(duration time.Duration) string {
	sb := strings.Builder{}
	fmt.Fprintf(&sb, "time spent: %v\n", duration)
	fmt.Fprintf(&sb, "payments sent: %d\n", s.paymentsSent)
	fmt.Fprintf(&sb, "payments received: %d\n", s.paymentsReceived)
	fmt.Fprintf(&sb, "payments tps: %.3f\n", float64(s.paymentsSent+s.paymentsReceived)/duration.Seconds())
	fmt.Fprintf(&sb, "buffered payments sent: %d\n", s.bufferedPaymentsSent)
	fmt.Fprintf(&sb, "buffered payments received: %d\n", s.bufferedPaymentsReceived)
	fmt.Fprintf(&sb, "buffered payments tps: %.3f\n", float64(s.bufferedPaymentsSent+s.bufferedPaymentsReceived)/duration.Seconds())
	return sb.String()
}
