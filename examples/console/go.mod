module github.com/stellar/experimental-paymentment-channels/examples/console

go 1.16

replace github.com/stellar/go => github.com/leighmcculloch/stellar--go v0.0.0-20210723165255-19672961f912

replace github.com/stellar/experimental-payment-channels/sdk => ../../sdk

require (
	github.com/stellar/experimental-payment-channels/sdk v0.0.0-00010101000000-000000000000
	github.com/stellar/go v0.0.0-00010101000000-000000000000
	gopkg.in/yaml.v2 v2.3.0 // indirect
)
