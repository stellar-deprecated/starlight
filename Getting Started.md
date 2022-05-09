# Getting Started

- [Testing with testnet](#testing-with-testnet)
- [Running your own devnet with CAP-21 and CAP-40](#running-your-own-devnet-with-cap-21-and-cap-40)
- [Experiment with the prototype Starlight Go SDK](#experiment-with-the-prototype-starlight-go-sdk)
- [Run the console example application](#run-the-console-example-application)
- [Manually inspect or build transactions](#manually-inspect-or-build-transactions)

## Testing with testnet

The Stellar testnet has protocol 19 implemented:

https://horizon-testnet.stellar.org

## Running your own devnet with CAP-21 and CAP-40

To run a standalone network use the branch of the `stellar/quickstart` docker image:

### Locally

```
docker run --rm -it -p 8000:8000 --name stellar stellar/quickstart:testing --standalone
```

Test that the network is running by curling the root endpoint:

```
curl http://localhost:8000
```

There is a friendbot included in the network that can faucet new accounts.

### Deployed

Alternatively, deploy a standalone network to Digital Ocean to develop and test with:

[![Deploy to DO](https://www.deploytodo.com/do-btn-blue.svg)](https://cloud.digitalocean.com/apps/new?repo=https://github.com/stellar/docker-stellar-core-horizon/tree/protocol19)

The network passphrase will be randomly generated in this case, and you can retrieve it from the logs.

_Reminder:_ The DigitalOcean server is publicly accessible on the Internet. Do not put sensitive information on the network that you would not want someone else to know. Anyone with access to the network will be able to use the root account above.

## Experiment with the prototype Starlight Go SDK

The `sdk` directory contains the prototype Starlight Go SDK.

Reference: https://pkg.go.dev/github.com/stellar/starlight/sdk

```
go get github.com/stellar/starlight/sdk
```

See the [console example](https://github.com/stellar/experimental-payment-channels/tree/readme/examples/console) application for an example for how to use the Starlight agent.

## Run the console example application

The `examples/console` directory contains an example application that operates a payment channel between two participants over a TCP connection. Requires a network to be running with support for CAP-21 and CAP-40.

See the [README](examples/console/README.md) for more details.

## Manually inspect or build transactions

You can use [stc](https://github.com/xdrpp/stc) to manually build and inspect transactions at the command line using text files.

Build transactions as you normally would, but for standalone network.
```
stc -net=standalone -edit tx
stc -net=standalone -sign tx | stc -net=standalone -post -
```
