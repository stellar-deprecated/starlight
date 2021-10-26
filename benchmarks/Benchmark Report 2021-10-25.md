# Benchmark Report 2021-10-25

This report is a summary of the results seen on 2021-10-25 and 2021-10-26
benchmarking a Starlight payment channel. This report details the setup and
how the benchmark was run, as well as the results.

The application used for the benchmarking is the `examples/console` application
in this repository. Some aspects of the application have been optimized, but
many parts have not. As an example, participants use simple JSON where some of the message is compressed. The results in this report should not be viewed as the
maximum possible performance possible with a Starlight payment channel.

## Summary

The test demonstrated that performance varies wildly and is influenced
significantly by the latency of the network connection between the payment
channel participants. However, even connections with greater latency, such
as those spanning countries, can deliver high numbers of payments per second.

The tests use a Starlight bi-directional payment channel, but only move
assets in a single direction for the duration of the test. The tests send
payments using buffering, where payments are buffered and sent in batches.

Test AB6 demonstrated over 400,000 payments per second for payments varying
from 0.0000001 to 1000.0, over Internet with 10-20ms latency.

Since smaller amounts results in smaller buffer sizes over the wire, higher
payments per second were observed for payments that trend towards
micro-payments.

For example, test AB5 demonstrated over 700,000 payments per second for
payments varying from 0.0000001 to 1.0. And, test AB2 over 1,190,000 payments per
second with micro-payments where every payment is for 0.0000001.

Tests AC1, AC2, AC3 also demonstrated that at least 17,000 to 130,000 payments per
second is possible on networks with latency as high as 215ms.

## Participants

Three participants were involved in the tests, with a payment channel formed
between participants A and B, and also between participants A and C.

### Participant A

- Location: San Francisco, US
- Network: 1gbps
- Hardware:
   ```
   MacBook Pro (13-inch, 2019, Four Thunderbolt 3 ports)
   2.8 GHz Quad-Core Intel Core i7
   16 GB 2133 MHz LPDDR3
   ```

### Participant B

- Location: San Francisco, US
- Network: ~200mbps
- Hardware:
   ```
   MacBook Pro (16-inch, 2019)
   2.6 GHz 6-Core Intel Core i7
   32 GB 2667 MHz DDR4
   ```

### Participant C

- Location: Florianopolis, Brazil
- Network: 200mbps
- Hardware:
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

Each test sends payments in one direction on the bi-directional payment channel. Each test specifies a max payment amount, payment count, and a maximum buffer size in the `payx` command. i.e. `payx <maximum payment amount> <payment count> <max buffer size>`.

For example, test AB6 `payx 1000 1000000 50000`, sent 1 million payments, with amounts varying from 0.0000001 to 1000.0, and it allowed payments to buffer into batches holding up to 50k payments.

Payments are buffered until the channel is available to propose the next agreement. An agreement is then built containing one or more buffered payments. The agreement containing a list of all buffered payments are transmitted to the recipient, and back again in full.

Agreements TPS is the number of agreements per second that the participants have signed and exchanged. Agreements are transmitted across the wire and so the agreements TPS is limited by the patency of both participants and their capability to quickly produce ed25519 signatures for the two transactions that form each agreement. Agreements TPS is limited by the latency between both participants. For example, if the participants have a ping time of 30ms, the highest agreements TPS value possible will be 1s/30ms=33tps. Or if the participants have a ping time of 200ms, the highest agreements TPS value possible will be 1s/200ms=5tps.

Buffered payments TPS is the number of buffered payments per second that the participants have exchanged inside agreements. Buffered payments are transmitted across the wire inside an agreement. The maximum buffer size is the maximum number of payments that will be transmitted inside a single agreement. Buffered payments TPS area limited by the bandwidth and latency between both participants.

Buffered payments average buffer size is the average size in bytes of the buffers that were transmitted between participants.

For more information on buffering payments, see this discussion: https://github.com/stellar/experimental-payment-channels/discussions/330#discussioncomment-1345257

#### Participants A (US) and B (US)

Latency between participants A and B was observed to be approximately 10-30ms.

##### Test AB1

Sending 3m payments, buffering up to 15k at a time, all payments with amount
0.0000001, all payments with unique memos.

```
> payx 0.0000001 3000000 15000
sending 0.0000001 payment 3000000 times
time spent: 4.063101492s
agreements sent: 201
agreements received: 0
agreements tps: 49.470
buffered payments sent: 3000000
buffered payments received: 0
buffered payments tps: 738352.218
buffered payments max buffer size: 55072
buffered payments min buffer size: 600
buffered payments avg buffer size: 48020
```

##### Test AB2

Sending 3m payments, buffering up to 95k at a time, all payments with amount
0.0000001, all payments with unique memos.

```
> payx 0.0000001 3000000 95000
sending 0.0000001 payment 3000000 times
time spent: 2.50396729s
agreements sent: 33
agreements received: 0
agreements tps: 13.179
buffered payments sent: 3000000
buffered payments received: 0
buffered payments tps: 1198098.718
buffered payments max buffer size: 349356
buffered payments min buffer size: 13380
buffered payments avg buffer size: 234642
```

##### Test AB3

```
> payx 1 3000000 15000
sending 1 payment 3000000 times
time spent: 5.898862686s
agreements sent: 201
agreements received: 0
agreements tps: 34.074
buffered payments sent: 3000000
buffered payments received: 0
buffered payments tps: 508572.611
buffered payments max buffer size: 153684
buffered payments min buffer size: 1980
buffered payments avg buffer size: 152424
```

##### Test AB4

```
> payx 1 3000000 1000
sending 1 payment 3000000 times
time spent: 39.129140064s
agreements sent: 3001
agreements received: 0
agreements tps: 76.695
buffered payments sent: 3000000
buffered payments received: 0
buffered payments tps: 76669.203
buffered payments max buffer size: 10292
buffered payments min buffer size: 1140
buffered payments avg buffer size: 9601
```

##### Test AB5

```
> payx 1 1000000 50000
sending 1 payment 1000000 times
time spent: 1.427511708s
agreements sent: 21
agreements received: 0
agreements tps: 14.711
buffered payments sent: 1000000
buffered payments received: 0
buffered payments tps: 700519.649
buffered payments max buffer size: 503908
buffered payments min buffer size: 1992
buffered payments avg buffer size: 502620
```

##### Test AB6

```
> payx 1000 1000000 50000
sending 1000 payment 1000000 times
time spent: 2.421638555s
agreements sent: 21
agreements received: 0
agreements tps: 8.672
buffered payments sent: 1000000
buffered payments received: 0
buffered payments tps: 412943.541
buffered payments max buffer size: 619884
buffered payments min buffer size: 138768
buffered payments avg buffer size: 549220
```

#### Participants A (US) and C (Brazil)

Latency between participants A and C was observed to be approximately 215ms.

##### Test AC1

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

##### Test AC2

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

##### Test AC3

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

##### Test AC4

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
