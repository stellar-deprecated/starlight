# Benchmark Report 2021-11-05

This report is a summary of the results seen on 2021-11-05 benchmarking a
Starlight payment channel. This report details the setup and how the benchmark
was run, as well as the results.

The application used for the benchmarking is the `examples/console` application
in this repository. Some aspects of the application have been optimized, but
many parts have not. The results in this report should not be viewed as the
maximum possible performance possible with a Starlight payment channel.

## Summary

The tests use a Starlight bi-directional payment channel, but only move assets
in a single direction for the duration of the test. The tests send payments
using buffering, where payments are buffered and sent in batches.

Test AB3 enabled snapshotting and demonstrated 1.1 million payments per second
for payments varying from 0.0000001 to 0.001.

Test AB2 disabled snapshotting and demonstrated similar, over 1.5 million
payments per second for payments varying from 0.0000001 to 0.001.

Test AB1 demonstrated over 1.5 million payments per second for payments varying
from 1 to 1000.0, over Internet with 20-30ms latency.

Test AB4 sent payments serially and was therefore limited by the network latency
between participants and demonstrated 38 payments and agreements per second.

## Participants

Two participants were involved in the tests, with a payment channel formed
between participants A and B.

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

## Setup

A private network running using Docker container from:
https://github.com/stellar/docker-stellar-core-horizon/tree/4d34da83d0876e21ff68cf90653053879239707b

Benchmark application in use `examples/console` from:
https://github.com/stellar/experimental-payment-channels/tree/9dbede9133883d6fcd69ca037c3e3466bad1f974

Note in some tests no snapshots of the channel state are being written to disk.
See each test for whether it was enabled or not. Tests that had it enabled used
the `-f` option included as optional below.

### Listener

```
$ cd examples/console
$ go run -horizon=<horizon> -listen=:8001 [-f snapshot.json]
```

### Connector

```
$ cd examples/console
$ go run -horizon=<horizon> -connect=<IP>:8001 [-f snapshot.json]
> open USD
> deposit 90000000000
```

## Tests and Results

### Understanding the Tests

Each test sends payments in one direction on the bi-directional payment channel.
Each test specifies a max payment amount, payment count, and a maximum buffer
size in the `payx` command. i.e. `payx <maximum payment amount> <payment count>
<max buffer size>`.

For example, test AB1 `payx 0.001 10000000 95000`, sent 1 million payments, with
amounts varying from 0.00000001 to 0.001, and it allowed payments to buffer into
batches holding up to 95k payments.

Payments are buffered until the channel is available to propose the next
agreement. An agreement is then built containing one or more buffered payments.
The agreement containing a list of all buffered payments are transmitted to the
recipient, and back again in full.

Agreements TPS is the number of agreements per second that the participants have
signed and exchanged. Agreements are transmitted across the wire and so the
agreements TPS is limited by the latency between both participants and their
capability to quickly produce ed25519 signatures for the two transactions that
form each agreement. For example, if the participants have a ping time of 30ms,
the highest agreements TPS value possible will be 1s/30ms=33tps.

Buffered payments TPS is the number of buffered payments per second that the
participants have exchanged inside agreements. Buffered payments are transmitted
across the wire inside an agreement. The maximum buffer size is the maximum
number of payments that will be transmitted in a single agreement. Buffered
payments TPS are limited by the bandwidth and latency between both participants.

Buffered payments average buffer size is the average size in bytes of the
buffers that were transmitted between participants.

For more information on buffering payments, see this discussion:
https://github.com/stellar/experimental-payment-channels/discussions/330#discussioncomment-1345257

#### Participants A (US) and B (US)

Latency between participants A and B was observed to be approximately 20-30ms.

##### Test AB1

Test AB1 ran 10 million payments of $0.001 or less, with payments being buffered
into buffers of max size 95,000. The application blocked and waited for the
response of the previous buffer before sending the next buffer.  12 buffers per
second and 1.1 million payments per second were witnessed. Each message was a
bit more than 309KB in size. Snapshots were enabled and as such the state of the
channel was being written to disk.

```
>>> payx 0.001 10000000 95000
sending 0.001 payment 10000000 times
time spent: 8.402299672s
agreements sent: 107
agreements received: 0
agreements tps: 12.735
buffered payments sent: 10000000
buffered payments received: 0
buffered payments tps: 1190150.362
buffered payments max buffer size: 490698
buffered payments min buffer size: 773
buffered payments avg buffer size: 309420
```

##### Test AB3

Test AB3 ran 10 million payments of $1000 or less, with payments being buffered
into buffers of max size 95,000. The application blocked and waited for the
response of the previous buffer before sending the next buffer.  12 buffers per
second and 1.1 million payments per second were witnessed. Each message was a
bit more than 485KB in size. Snapshots were not enabled and as such the state of
the channel was not being written to disk.

```
>>> payx 1000 10000000 95000
sending 1000 payment 10000000 times
time spent: 6.444609094s
agreements sent: 107
agreements received: 0
agreements tps: 16.603
buffered payments sent: 10000000
buffered payments received: 0
buffered payments tps: 1551684.494
buffered payments max buffer size: 485915
buffered payments min buffer size: 768
buffered payments avg buffer size: 306113
```

##### Test AB2

Test AB2 ran 10 million payments of $0.001 or less, with payments being buffered
into buffers of max size 95,000. The application blocked and waited for the
response of the previous buffer before sending the next buffer.  12 buffers per
second and 1.1 million payments per second were witnessed. Each message was a
bit more than 490KB in size. Snapshots were not enabled and as such the state of
the channel was not being written to disk.

```
>>> payx 0.001 10000000 95000
sending 0.001 payment 10000000 times
time spent: 6.429692082s
agreements sent: 107
agreements received: 0
agreements tps: 16.642
buffered payments sent: 10000000
buffered payments received: 0
buffered payments tps: 1555284.432
buffered payments max buffer size: 490652
buffered payments min buffer size: 544
buffered payments avg buffer size: 309526
```

##### Test AB4

Test AB4 ran 1000 payments of $1 or less, with each payment being sent serially
over the network. The application blocked and waited for the response before
sending the next payment. 38 payments per second were witnessed. Each message
was a bit more than 190 bytes in size. Snapshots were not enabled and as such
the state of the channel was not being written to disk.

```
>>> payx 1 1000 1
sending 1 payment 1000 times
time spent: 26.247297693s
agreements sent: 1000
agreements received: 0
agreements tps: 38.099
buffered payments sent: 1000
buffered payments received: 0
buffered payments tps: 38.099
buffered payments max buffer size: 190
buffered payments min buffer size: 182
buffered payments avg buffer size: 188
```
