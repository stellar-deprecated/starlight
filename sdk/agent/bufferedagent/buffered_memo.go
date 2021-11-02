package bufferedagent

import (
	"bytes"
	"encoding/gob"
	"fmt"
	// "github.com/klauspost/compress/gzip"
)

type bufferedPaymentsMemo struct {
	ID       string
	Payments []BufferedPayment
}

func (m *bufferedPaymentsMemo) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	// z, err := gzip.NewWriterLevel(&b, gzip.BestSpeed)
	// if err != nil {
	// 	panic(fmt.Errorf("creating gzip writer: %w", err))
	// }
	enc := gob.NewEncoder(&b)
	type bpm bufferedPaymentsMemo
	err := enc.Encode((*bpm)(m))
	if err != nil {
		return nil, fmt.Errorf("encoding buffered payments memo: %w", err)
	}
	// z.Close()
	return b.Bytes(), nil
}

func (m *bufferedPaymentsMemo) UnmarshalBinary(b []byte) error {
	r := bytes.NewReader(b)
	// z, err := gzip.NewReader(r)
	// if err != nil {
	// 	return fmt.Errorf("creating gzip reader: %w", err)
	// }
	dec := gob.NewDecoder(r)
	type bpm bufferedPaymentsMemo
	err := dec.Decode((*bpm)(m))
	if err != nil {
		return fmt.Errorf("decoding buffered payments memo: %w", err)
	}
	return nil
}
