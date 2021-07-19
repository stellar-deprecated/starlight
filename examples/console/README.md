# Console Example

This module contains an example using the SDK in this repository that supports a
single payment channel controlled by two people through a command line interface
using simple TCP network connection and JSON messages.

## State

This example is a rough example and is brittle and is still a work in progress.

## Install

```
go install github.com/stellar/experimental-payment-channels/examples/console@latest
```

Or clone this repository:

```
git clone https://github.com/stellar/experimental-payment-channels
cd examples/console
```

## Usage

Follow the [Getting Started](../../Getting Started.md) instructions to run a
stellar-core and horizon with CAP-21 capabilities.

Run the example providing a key and its secret key.

```
console -horizon=http://localhost:8000 -account=G... -signer=S...
```

The application will create an escrow account. Type `help` once in to discover
commands to use.
