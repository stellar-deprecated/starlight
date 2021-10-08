package bufferedagent

import (
	"encoding/json"
	"fmt"
)

type settlementMemo struct {
	ID      string
	Amounts []int64
}

func (m settlementMemo) String() string {
	memoBytes, err := json.Marshal(m)
	if err != nil {
		panic(fmt.Errorf("encoding settlement memo as json: %w", err))
	}
	return string(memoBytes)
}

func parseSettlementMemo(memo string) (settlementMemo, error) {
	m := settlementMemo{}
	err := json.Unmarshal([]byte(memo), &m)
	if err != nil {
		return settlementMemo{}, fmt.Errorf("decoding settlement memo from json: %w", err)
	}
	return m, nil
}
