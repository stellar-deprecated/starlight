# Benchmark Report 2021-10-25

Author: Leigh McCulloch <@leighmcculloch>

This report is a summary of the results seen on 2021-10-25 benchmarking a
Starlight payment channel. This report details the setup and how the benchmark
was run, as well as the results.

The application used for the benchmarking is the `examples/console` application
in this repository. Some aspects of the application have been optimized, but
many parts have not. The results in this report should not be viewed as the
maximum possible performance possible with a Starlight payment channel.

## Summary

The test demonstrated that at least ____ transactions per second is possible
with consumer hardware when using a Starlight payment channel and buffering
payments.

## Hardware

The two participants were using the following hardware:

```
MacBook Pro (13-inch, 2019, Four Thunderbolt 3 ports)
2.8 GHz Quad-Core Intel Core i7
16 GB 2133 MHz LPDDR3
```

```
???
2.6 GHz 6-Core Intel Core i7
???
```

## Network

Participants were connected over Tailscale using 1gbps and ____ speed networks.

Participants witnessed a __ms ping time between their computers.

## Setup

Test network running using Docker container from:
https://github.com/stellar/docker-stellar-core-horizon/tree/4d34da83d0876e21ff68cf90653053879239707b

Benchmark application in use `examples/console` from:
https://github.com/stellar/experimental-payment-channels/tree/8f8f9c2721d19f9e8af4cfa783d2d2efcaf2de97

### Listener

```
$ cd examples/console
$ go run -horizon=<horizon> -listen=:8001
```

### Connector

```
$ cd examples/console
$ go run -horizon=<horizon> -http=9000 -connect=<IP>:8001
> open USD
```

## Tests and Results

### Test 1
Sending 10m payments, buffering up to 95k at a time, all payments with amount
0.0000001, all payments with unique memos.

```
> deposit 10000000
> deposit 10000000 other
> payx 0.0000001 10000000 95000
```

```
// TODO: OUTPUT HERE
```

### Test 2
Sending 10m payments, buffering up to 95k at a time, payment amounts randomly
selected in the range 100.0 to 0.0000001, all payments with unique memos.

```
> deposit 10000000
> deposit 10000000 other
> payx 100.0 10000000 95000
```

```
// TODO: OUTPUT HERE
```
