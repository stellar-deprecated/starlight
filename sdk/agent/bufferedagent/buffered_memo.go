package bufferedagent

import (
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"strings"
)

type bufferedPaymentsMemo struct {
	ID       string
	Payments []BufferedPayment
}

func (m bufferedPaymentsMemo) String() string {
	sb := strings.Builder{}
	// b64 := base64.NewEncoder(base64.StdEncoding, &sb)
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
	return sb.String()
}

func parseBufferedPaymentMemo(memo string) (bufferedPaymentsMemo, error) {
	r := strings.NewReader(memo)
	// b64 := base64.NewDecoder(base64.StdEncoding, r)
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
