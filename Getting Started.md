# Getting Started

- [Connecting to the deployed devnet](#connecting-to-the-deployed-devnet)
- [Running your own devnet with CAP-21 and CAP-40](#running-your-own-devnet-with-cap-21-and-cap-40)
- [Experiment with the prototype Starlight Go SDK](#experiment-with-the-prototype-starlight-go-sdk)
- [Run the console example application](#run-the-console-example-application)
- [Manually inspect or build transactions](#manually-inspect-or-build-transactions)

## Connecting to the deployed devnet

The deployed devnet is a development network with no uptime or data durability
guarantees. It is intended for convenient use with examples or testing. A
network reset occurs when deployed.

Horizon: https://horizon-devnet-cap21and40.stellar.org  
Network Passphrase: `Starlight Network; October 2021`

To run the example console application with the deployed devnet:

```
git clone https://github.com/stellar/starlight
cd examples/console
go run . -horizon=http://horizon-devnet-cap21and40.stellar.org
>>> help
```

Run two copies of the example console application and connect them directly over
TCP to open a payment channel between two participants.

## Running your own devnet with CAP-21 and CAP-40

To run a standalone network use the branch of the `stellar/quickstart` docker image:

### Locally

```
docker build -t stellar/quickstart:cap21and40 git://github.com/stellar/docker-stellar-core-horizon#cap21and40
```

```
docker run --rm -it -p 8000:8000 --name stellar stellar/quickstart:cap21and40 --standalone
```

The root account of the network will be:
```
GBZXN7PIRZGNMHGA7MUUUF4GWPY5AYPV6LY4UV2GL6VJGIQRXFDNMADI
SC5O7VZUXDJ6JBDSZ74DSERXL7W3Y5LTOAMRF7RQRL3TAGAPS7LUVG3L
```

There is no friendbot, so you'll need to create a transaction funding accounts from the root account.

Test that the network is running by curling the root endpoint:

```
curl http://localhost:8000
```

### Deployed

Alternatively, deploy a standalone network to Digital Ocean to develop and test with:

[![Deploy to DO](https://www.deploytodo.com/do-btn-blue.svg)](https://cloud.digitalocean.com/apps/new?repo=https://github.com/stellar/docker-stellar-core-horizon/tree/cap21and40)

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

See the [README](https://github.com/stellar/starlight/tree/readme/examples/console) for more details.

## Manually inspect or build transactions

You can use `stc` to manually build and inspect transactions at the command line using text files. A fork of `stc` has been updated to support CAP-21 and CAP-40. Use the instructions below to install it.

```
git clone -b cap21 https://github.com/leighmcculloch/xdrpp--stc stc
cd stc
make depend
make
make install
export STCNET=standalone
```

Build transactions as you normally would, but for standalone network.
```
stc -edit tx
stc -sign tx | stc -post -
```
