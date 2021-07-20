# Console Example

This module contains an example using the SDK in this repository that supports a
single payment channel controlled by two people through a command line interface
using simple TCP network connection and JSON messages.

## State

This example is a rough example and is brittle and is still a work in progress.

## Usage

Follow the [Getting Started](../../Getting%20Started.md) instructions to run a
stellar-core and horizon with CAP-21 capabilities.

Run the example providing a G strkey and its S strkey secret key. Generate your
own keys using stc or the Stellar Laboratory.

```
git clone https://github.com/stellar/experimental-payment-channels
cd examples/console
go run . -horizon=http://localhost:8000 -account=GCIFBFIALD3GDBNHAA7QHOYMYFEXVZTRY6ICHWDDB5HELJRGVJZ4HB5V -signer=SDD7AE7ZFFXWC4LNAILC5XQCV7J5BHGUJULB2MAHQ4SQS5SCUYG2YFUW
```

The application will create the account passed in if needed.

The application will randomly generate and create an escrow account as well.

Type `help` once in to discover commands to use.

## Example

Run these commands on the first terminal:
```
$ go run . -horizon=http://localhost:8000 -account=GCIFBFIALD3GDBNHAA7QHOYMYFEXVZTRY6ICHWDDB5HELJRGVJZ4HB5V -signer=SDD7AE7ZFFXWC4LNAILC5XQCV7J5BHGUJULB2MAHQ4SQS5SCUYG2YFUW
> listen :6000
> open
> deposit 100
> pay 4
> pay 46
```

Run these commands in the second terminal:
```
$ go run . -horizon=http://localhost:8000 -account=GBNQ3V2QI47IG5RCSEUF3FDDNYZWWYIKNYBFO3XF2BUV6DZWOEJBM66P -signer=SA5TQFNYZSOSVOIYKKQZ5DLSWS4JEGLR4FACLGQD7DRXDOGWK6N3ABCI
> connect :6000
> deposit 80
> pay 2
> close
```
