## Preamble

```
SEP: ????
Title: Payment Channel
Author: Leigh McCulloch <@leighmcculloch>
Track: Standard
Status: Draft
Discussion: https://github.com/stellar/experimental-payment-channels/issues
Created: 2021-04-21
Updated: 2021-04-21
Version: 0.0.1
```

## Summary

This protocol defines the transactions that two participants use to open,
mutate, and close a payment channel.

## Dependencies

This protocol is dependent on [CAP-21] and is built on the example two-way
payment channel protocol defined in that CAP's rationale.

This protocol is also dependent on [CAP-23] providing claimable balances and
[CAP-33] providing sponsorship.

## Motivation

Stellar account holders who frequently interact with a set of partners must
choose between performing every transaction on-chain, or trusting each other
to some degree and performing net-settlement. Transactions on-chain are fast
and affordable but it would be beneficial to some operators if speed and cost
were not bounded and only limited by the partners own capability to agree.

## Abstract

This protocol defines...

## Use Cases


## Specification

TODO

## Security Concerns

TODO

## Limitations

This protocol defines the mechanisms of the Stellar network's core protocol
that are used to enforce agreements made by two participants. This protocol
does not define the transport through which the agreements are coordinated,
or the methods through which more than two participants can coordinate and
exchange dependent agreements. These issues are likely to be discussed in a
separate proposal.

## Implementations

TODO

[CAP-21]: https://stellar.org/protocol/cap-21
[CAP-23]: https://stellar.org/protocol/cap-23
[CAP-33]: https://stellar.org/protocol/cap-33
