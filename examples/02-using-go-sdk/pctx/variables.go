package pctx

const m = 2

func s_i(s int64, i int) int64 {
	return s + int64(i)*m
}

func s_e(s int64, e int) int64 {
	return s + int64(e)*m
}
