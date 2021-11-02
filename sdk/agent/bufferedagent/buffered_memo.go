package bufferedagent

import (
	"encoding/gob"
	"fmt"
	"strings"

	// "github.com/klauspost/compress/gzip"
)

type bufferedPaymentsMemo struct {
	ID       string
	Payments []BufferedPayment
}

func (m bufferedPaymentsMemo) String() string {
	sb := strings.Builder{}
	z := &sb
	// z, err := gzip.NewWriterLevel(&sb, gzip.BestSpeed)
	// if err != nil {
	// 	panic(fmt.Errorf("creating gzip writer: %w", err))
	// }
	enc := gob.NewEncoder(z)
	err := enc.Encode(m)
	if err != nil {
		panic(fmt.Errorf("encoding buffered payments memo as json: %w", err))
	}
	// z.Close()
	return sb.String()
}

func parseBufferedPaymentMemo(memo string) (bufferedPaymentsMemo, error) {
	r := strings.NewReader(memo)
	z := r
	// z, err := gzip.NewReader(r)
	// if err != nil {
	// 	return bufferedPaymentsMemo{}, fmt.Errorf("creating gzip reader: %w", err)
	// }
	dec := gob.NewDecoder(z)
	m := bufferedPaymentsMemo{}
	err := dec.Decode(&m)
	if err != nil {
		return bufferedPaymentsMemo{}, fmt.Errorf("decoding buffered payments memo from json: %w", err)
	}
	return m, nil
}
