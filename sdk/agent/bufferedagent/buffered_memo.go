package bufferedagent

import (
	"encoding/json"
	"fmt"
)

type bufferedPaymentsMemo struct {
	ID      string
	Amounts []int64
}

func (m bufferedPaymentsMemo) String() string {
	memoBytes, err := json.Marshal(m)
	if err != nil {
		panic(fmt.Errorf("encoding buffered payments memo as json: %w", err))
	}
	return string(memoBytes)
}

func parseBufferedPaymentMemo(memo string) (bufferedPaymentsMemo, error) {
	m := bufferedPaymentsMemo{}
	err := json.Unmarshal([]byte(memo), &m)
	if err != nil {
		return bufferedPaymentsMemo{}, fmt.Errorf("decoding buffered payments memo from json: %w", err)
	}
	return m, nil
}
