package bufferedagent

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/klauspost/compress/gzip"
)

type bufferedPaymentsMemo struct {
	ID       string
	Payments []BufferedPayment
}

func (m bufferedPaymentsMemo) Bytes() []byte {
	sb := bytes.Buffer{}
	z, err := gzip.NewWriterLevel(&sb, gzip.BestSpeed)
	if err != nil {
		panic(fmt.Errorf("creating gzip writer: %w", err))
	}
	enc := gob.NewEncoder(z)
	err = enc.Encode(m)
	if err != nil {
		panic(fmt.Errorf("encoding buffered payments memo as json: %w", err))
	}
	z.Close()
	return sb.Bytes()
}

func parseBufferedPaymentMemo(memo []byte) (bufferedPaymentsMemo, error) {
	r := bytes.NewReader(memo)
	z, err := gzip.NewReader(r)
	if err != nil {
		return bufferedPaymentsMemo{}, fmt.Errorf("creating gzip reader: %w", err)
	}
	dec := gob.NewDecoder(z)
	m := bufferedPaymentsMemo{}
	err = dec.Decode(&m)
	if err != nil {
		return bufferedPaymentsMemo{}, fmt.Errorf("decoding buffered payments memo from json: %w", err)
	}
	return m, nil
}
