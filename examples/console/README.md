# Console Example

This module contains an example using the SDK in this repository that supports a
single payment channel controlled by two people through a command line interface
using simple TCP network connection and JSON messages.

## State

This example is a rough example and is brittle and is still a work in progress.

## Usage

Run the example using the below commands.

```
git clone https://github.com/stellar/starlight
cd examples/console
go run .
```

Type `help` once in to discover commands to use.

Foreground events and commands are written to stdout. Background events are
written to stderr. To view them side-by-side pipe them to a file or alternative
tty device.

## Example

Run these commands on the first terminal:
```
$ go run .
> listen :6000
> open
> deposit 100
> pay 4
> pay 46
```

Run these commands in the second terminal:
```
$ go run .
> connect :6000
> deposit 80
> pay 2
> declareclose
> close
```
