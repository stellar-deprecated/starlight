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

This protocol defines the Stellar transactions that two participants use to
open and close a payment channel.

## Dependencies

This protocol is dependent on [CAP-21] and is based on the two-way payment
channel protocol defined in that CAP's rationale.

This protocol is also dependent on [CAP-23] that added claimable balances
ledger entries and [CAP-33] that added sponsorship to accounts.

## Motivation

Stellar account holders who frequently transact with each other, but do not
trust each other, perform all their transactions on-chain to get the benefits
of the network enforcing transaction finality. The network is fast, but not
as fast as two parties forming an agreement directly with each other. For
high-frequency transactors it would be beneficial if there was a simple
method on Stellar to use the concepts found in payment channels to allow two
parties to escrow funds into an account that is controlled by both parties,
where agreements can be formed and guaranteed to be executable and contested
on-chain. [CAP-21] introduces new fields that make it easier to do this.

## Abstract

TODO

## Specification

A payment channel has two participants, an initiator I and a responder R.

The protocol assumes some _synchrony period_, S, such that both parties are
guaranteed to be able to observe the blockchain state and submit transactions
within any period of length S.

The payment channel consists of a 2-of-2 multisig escrow account E, initially
created and configured by I, and a series of transaction sets that contain
_declaration_ and _closing_ transactions on E signed by both parties. The
closing transaction defines the final state of the escrow account and the
assets it holds.

The two parties maintain the following two variables during the lifetime of
the channel:

* s - the _starting sequence number_, is initialized to one greater
than the sequence number of the escrow account E after E has been created and
configured. It is increased only when withdrawing from or topping up the
escrow account E.

* i - the _iteration number_ of the payment channel, is initialized to
(s/2)+1. It is incremented with every off-chain update of the payment channel
state.

To update the payment channel state, the parties:

1. Increment i.
2. Sign and exchange two closing transactions IC_i and RC_i.
3. Sign and exchange a declaration transaction D_i.

The transactions are constructed as follows:

* D_i, the _declaration transaction_, declares an intent to execute
the corresponding closing transaction C_i. D_i has source account E, sequence
number 2i, and `minSeqNum` set to s. Hence, D_i can execute at any time, so
long as E's sequence number n satisfies s <= n < 2i. D_i always leaves E's
sequence number at 2i after executing. Because C_i has source account E and
sequence number 2i+1, D_i leaves E in a state where C_i can execute. Note
that D_i does not require any operations, but since Stellar disallows empty
transactions, it contains a `BUMP_SEQUENCE` operation as a no-op.

* C_i, the _closing transaction_, disburses funds to R and changes the
signing weights on E such that I unilaterally controls E. C_i has source
account E, sequence number 2i+1, and a `minSeqAge` of S (the synchrony
period). The `minSeqAge` prevents a misbehaving party from executing C_i when
the channel state has already progressed to a later iteration number, as the
other party can always invalidate C_i by submitting D_i' for some i' > i. C_i
contains one or more `CREATE_CLAIMABLE_BALANCE` operations disbursing funds
to R, plus a `SET_OPTIONS` operation adjusting signing weights to give I full
control of E.

For R to top-up or withdraw excess funds from the escrow account E, the
participants skip a generation. They set s = 2(i+1), and i = i+2. They then
exchange C_i and D_i (which unlike the update case, can be exchanged in a
single phase of communication because D_i is not yet executable while E's
sequence number is below the new s). Finally, they create a top-up
transaction that atomically adjusts E's balance and uses `BUMP_SEQUENCE` to
increase E's sequence number to s.

To close the channel cooperatively, the parties re-sign C_i with a
`minSeqNum` of s and a `minSeqAge` of 0, then submit this transaction.

## Security Concerns

TODO

## Limitations

This protocol defines the mechanisms of the Stellar network's core protocol
that are used to enforce agreements made by two participants. This protocol
does not define the transport through which the agreements are coordinated,
or the methods through which more than two participants can coordinate and
exchange dependent agreements. These issues are likely to be discussed in
separate proposals.

## Implementations

TODO

[CAP-21]: https://stellar.org/protocol/cap-21
[CAP-23]: https://stellar.org/protocol/cap-23
[CAP-33]: https://stellar.org/protocol/cap-33
