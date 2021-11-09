# Getting Started

- [Running a network with CAP-21 and CAP-40](#running-core-and-horizon-with-cap-21-and-cap-40)
- [Experiment with the prototype Starlight Go SDK](#experiment-with-the-prototype-starlight-go-sdk)
- [Run the console example application](#run-the-console-example-application)
- [Manually build transactions](#manually-build-transactions)

## Running a network with CAP-21 and CAP-40

To run a standalone network use the branch of the `stellar/quickstart` docker image:

```
docker build -t stellar/quickstart:cap21and40 https://github.com/stellar/docker-stellar-core-horizon.git#cap21and40
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

## Experiment with the prototype Starlight Go SDK

The `sdk` directory contains the prototype Starlight Go SDK.

### Docs

https://pkg.go.dev/github.com/stellar/starlight/sdk

### Add to your Go project

```
go get github.com/stellar/starlight/sdk
```

## Run the console example application

The `examples/console` directory contains an example application that operates a payment channel between two participants over a TCP connection. Requires a network to be running with support for CAP-21 and CAP-40.

See the [README](https://github.com/stellar/starlight/tree/readme/examples/console) for more details.

## Manually build transactions

You can use `stc` to manually build transactions at the command line using text files. A fork of `stc` has been updated to support CAP-21 and CAP-40. Use the instructions below to install it.

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
