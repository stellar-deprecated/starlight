module github.com/stellar/experimental-payment-channels/examples/02-using-go-sdk

go 1.16

require (
	github.com/go-chi/chi v4.1.2+incompatible // indirect
	github.com/go-errors/errors v1.4.0 // indirect
	github.com/gorilla/schema v1.2.0 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.3 // indirect
	github.com/lib/pq v1.10.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/stellar/experimental-payment-channels/sdk v0.0.0-00010101000000-000000000000
	github.com/stellar/go v0.0.0-20210519164200-dc27694bbdb3
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/stretchr/testify v1.7.0 // indirect
	golang.org/x/sys v0.0.0-20210514084401-e8d321eab015 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/stellar/go => github.com/leighmcculloch/stellar--go v0.0.0-20210514225055-eafef939d8d2

replace github.com/stellar/experimental-payment-channels/sdk => ../../sdk
