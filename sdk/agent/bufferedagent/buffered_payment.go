package bufferedagent

type BufferedPayment struct {
	Amount int64  `json:",empty"`
	Memo   string `json:",empty"`
}
