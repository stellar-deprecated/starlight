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

The test demonstrated that at least \____ transactions per second is possible
with consumer hardware when using a Starlight payment channel and buffering
payments.

## Participants

Three participants were involved in the tests, with two participant sets formed
from participants A and B, and participants A and C.

### Participant A

Location: San Francisco, US
Network: 1gbps
Hardware:
```
MacBook Pro (13-inch, 2019, Four Thunderbolt 3 ports)
2.8 GHz Quad-Core Intel Core i7
16 GB 2133 MHz LPDDR3
```

### Participant B

Location: ?, US
Network: ?
Hardware:
```
???
2.6 GHz 6-Core Intel Core i7
???
```

### Participant C

Location: Florianopolis, Brazil
Network: 200mbps
Hardware:
```
MacBook Air (M1, 2020)
Apple M1
16 GB
```

## Setup

Test network running using Docker container from:
https://github.com/stellar/docker-stellar-core-horizon/tree/4d34da83d0876e21ff68cf90653053879239707b

Benchmark application in use `examples/console` from:
https://github.com/stellar/experimental-payment-channels/tree/f43363c6b1593134344629b32883529f494bab23

Note in both setups below no snapshots of the channel state are being written to disk. Data persistence functionality exists in the console application and can be enabled, but was not enabled for these tests. However, no significant difference in performance has been witnessed with having that functionality enabled.

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
> deposit 10000000
> deposit 10000000 other
```

## Tests and Results

### Understanding the Tests

Each test sends payments in one direction on the bi-directional payment channel. Each test specifies a payment count, max payment amount, and a maximum buffer size in the `payx` command. i.e. `payx <maximum payment amount> <payment count> <max buffer size>`.

Payments are buffered until the channel is available to propose the next agreement. An agreement is then built containing one or more buffered payments. The agreement containing a list of all buffered payments are transmitted to the recipient, and back again in full.

Agreements TPS is the number of agreements per second that the participants have signed and exchanged.

Buffered payments TPS is the number of buffered payments per second that the participants have exchanged inside agreements.

Buffered payments average buffer size is the average size in bytes of the buffers that were transmitted between participants.

For more information on buffering payments, see this discussion: https://github.com/stellar/experimental-payment-channels/discussions/330#discussioncomment-1345257

#### Participants A (US) and C (Brazil)

##### Test 1

Sending 3m payments, buffering up to 15k at a time, all payments with amount
0.0000001, all payments with unique memos.

```
> payx 0.0000001 3000000 15000
sending 0.0000001 payment 3000000 times
time spent: 1m28.168218631s
agreements sent: 201
agreements received: 0
agreements tps: 2.280
buffered payments sent: 3000000
buffered payments received: 0
buffered payments tps: 34025.866
buffered payments max buffer size: 55144
buffered payments min buffer size: 1520
buffered payments avg buffer size: 47475
```

##### Test 2

Sending 3m payments, buffering up to 95k at a time, all payments with amount
0.0000001, all payments with unique memos.

```
> payx 0.0000001 3000000 95000                                
sending 0.0000001 payment 3000000 times
time spent: 22.827299992s
agreements sent: 33
agreements received: 0
agreements tps: 1.446
buffered payments sent: 3000000
buffered payments received: 0
buffered payments tps: 131421.587
buffered payments max buffer size: 349288
buffered payments min buffer size: 952
buffered payments avg buffer size: 240436
```

##### Test 3

Sending 3m payments, buffering up to 15k at a time, payments with varying amounts ranging from 1.0 to
0.0000001, all payments with unique memos.

```
> payx 1 3000000 15000
sending 1 payment 3000000 times
time spent: 2m47.082713s
agreements sent: 201
agreements received: 0
agreements tps: 1.203
buffered payments sent: 3000000
buffered payments received: 0
buffered payments tps: 17955.179
buffered payments max buffer size: 153592
buffered payments min buffer size: 1648
buffered payments avg buffer size: 152538
```

##### Test 3

Sending 3m payments, buffering up to 15k at a time, payments with varying amounts ranging from 1.0 to
0.0000001, all payments with unique memos.

```
> payx 1 3000000 1000 
sending 1 payment 3000000 times
time spent: 12m37.733772304s
agreements sent: 3001
agreements received: 0
agreements tps: 3.960
buffered payments sent: 3000000
buffered payments received: 0
buffered payments tps: 3959.174
buffered payments max buffer size: 10484
buffered payments min buffer size: 3376
buffered payments avg buffer size: 6816
```
