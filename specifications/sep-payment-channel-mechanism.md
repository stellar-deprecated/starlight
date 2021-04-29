## Preamble

```
SEP: ????
Title: Payment Channel
Author: David Mazeries <@standford-scs>, Leigh McCulloch <@leighmcculloch>
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

This protocol is dependent on the not-yet-impemented [CAP-21], and is based
on the two-way payment channel protocol defined in that CAP's rationale.

This protocol is also dependent on [CAP-23], that added claimable balances
ledger entries, and [CAP-33], that added sponsorship to accounts.

## Motivation

Stellar account holders who frequently transact with each other, but do not
trust each other, are limited to performing all their transactions on-chain
to get the benefits of the network enforcing transaction finality.  The
network is fast, but not as fast as two parties forming an agreement directly
with each other.  For high-frequency transactors it would be beneficial if
there was a simple method on Stellar to allow two parties to escrow funds
into an account that is controlled by both parties, where agreements can be
formed and guaranteed to be executable and contested on-chain.  [CAP-21],
[CAP-23], and [CAP-33] introduce new functionality to the Stellar protocol
that make it easier to do this.

## Abstract

This protocol defines the Stellar transactions that two participants use to
open and close a payment channel by using an escrow account to holds funds, a
reserve account to hold native asset to pay for new ledger entries, and
claimable balances as an uninterruptable method for final disbursement.

A payment channel has two participants, an initiator I and a responder R.

The protocol assumes some _observation period_, O, such that both parties are
guaranteed to be able to observe the blockchain state and submit transactions
within any period of length O.

The payment channel consists of two 2-of-2 multisig escrow account E, and a
series of transaction sets that contain _declaration_ and _closing_
transactions on E signed by both participants.  The closing transaction
defines the final state of the channel that creates claimable balances for R
and returns control of E to I.  Each generation of declaration and closing
transaction sets in the series are an agreement on a new final state for the
channel.

The payment channel also uses a second 2-of-2 multisig reserve account V, to
sponsor the claimable balances ledger entries that are created at channel
close, and that disburse funds to R.

## Specification

### Participants

A payment channel has two participants:

- I, the _initiator_, who proposes the payment channel, performs the first
setup step, and will be able to make deposits to the payment channel without
coordination.  I creates escrow account E and receives disbursement through
regaining control of E at channel close.

- R, the _responder_, who joins the payment channel, and receives disbursement through claimable balances at channel close.

### Observation Period

A payment channel defines an observation period O within which all
participants are guaranteed to be able to observe the blockchain state and
submit transactions in response to changing state.

The participants agree on the period O at channel open.

The participants may agree at anytime by following the [Change the
Observation Period](#Change-the-Observation-Period) process.

#### Change the Observation Period

The participants may agree at anytime to decrease period O by simply using a
smaller value for O in future transaction sets.  The change will only apply
to future transaction sets.  The change does not require submitting a
transaction to the network.

The participants may agree at anytime to increase period O by using a larger
value for O in the next and future transaction sets, or regenerating the most
recent transaction set, then signing and submitting a transaction that bumps
the sequence number of the escrow account to the sequence before the most
recent D_i.  The sequence bump ensures only the most recent transaction with
the new period O is valid.

The participants:

1. Follow the [Update](#Update) process with the new period O.
2. Sign and exchange a bump transaction B.
3. TODO: Set new values for s and i.

The transactions are constructed as follows:

- B, the _bump transaction_, bumps the sequence number of escrow account E
such that only the most recent transaction set is valid.  B has source
account E, sequence number s.

  B contains operations:
  - One `BUMP_SEQUENCE` operation with sequence set to 2(i-1)+1.

### Accounts

The payment channel utilizes two Stellar accounts that are both 2-of-2
multisig accounts while the channel is open:

- E, the _escrow account_, that holds the assets that both participants have
contributed to the channel and that will be distributed to the participants
at channel close according to the final close transactions submitted.
Created by I.  Jointly controlled by I and R while the channel is open.
Control is returned to I at close.

- V, the _reserve account_, that holds an amount of native asset contributed
by R that will be used to sponsor the claimable balance ledger entries
created at disbursement.  Created by R.  Jointly controlled by I and R while
the channel is open.  Control is returned to R at close.  Cannot be merged
until all claimable balances created at close for R are claimed by R.

### Variables

The two participants maintain the following two variables during the lifetime
of the channel:

- s, the _starting sequence number_, is initialized to one greater
than the sequence number of the escrow account E after E has been created and
configured. It is changed only when withdrawing from or topping up the
escrow account E.

- i, the _iteration number_ of the payment channel, is initialized to
((s+1)/2)+1.  It is incremented with every off-chain update of the payment
channel state.

TODO: Change how the iteration number is initialized and used.

### Processes

#### Setup

To setup the payment channel:

1. I creates the escrow account E.
2. R creates the reserve account V.
3. I and R sign and exchange signatures for the first closing transaction C_i.
4. I and R sign and exchange signatures for the first declaration transaction D_i.
5. I and R sign and exchange signatures for the formation transaction F.

The transactions are constructed as follows:

- C_i, see [Update Process](#Update-Process).

- D_i, see [Update Process](#Update-Process).

- F, the _formation transaction_, deposits R's contribution to escrow account
E, and changes escrow account E and reserve account V to be 2-of-2 multisig
accounts.  F has source account E, and sequence number set to s.

  F contains operations:

  - Operations sponsored by I:
    - One `BEGIN_SPONSORING_FUTURE_RESERVES` operation that specifies participant
    I as a sponsor of future reserves.
    - One or more `SET_OPTIONS` operations adjusting escrow account E's
    thresholds such that I and R's signers must both sign, and adding I's
    signers to E.
    - One or more `SET_OPTIONS` operations adding I's signers to V.
    - One or more `CHANGE_TRUST` operations adding trustlines to E.
    - One `END_SPONSORING_FUTURE_RESERVES` operation that stops I sponsoring
    future reserves of subsequent operations.
  - Operations sponsored by R:
    - One `BEGIN_SPONSORING_FUTURE_RESERVES` operation that specifies reserve
    account R as a sponsor of future reserves.
    - One or more `SET_OPTIONS` operations adjusting escrow account V's
    thresholds such that R and I's signers must both sign, and adding R's
    signers to V.
    - One or more `SET_OPTIONS` operations adding R's signers to E.
    - One `END_SPONSORING_FUTURE_RESERVES` operation that stops R sponsoring
    future reserves of subsequent operations.
  - One or more `PAYMENT` operations depositing I's contribution to E.
  - One or more `PAYMENT` operations depositing R's contribution to E.
  - One or more `PAYMENT` operations depositing R's reserves for each
  trustline on E to V.

#### Update

To update the payment channel state, the participants:

1. Increment i.
2. Sign and exchange a closing transactions C_i.
3. Sign and exchange a declaration transaction D_i.

The transactions are constructed as follows:

- C_i, the _closing transaction_, disburses funds to R and changes the
signing weights on E such that I unilaterally controls E.  C_i has source
account E, sequence number 2i+1, and a `minSeqAge` of O (the observation
period).

  The `minSeqAge` prevents a misbehaving party from executing C_i when the
  channel state has already progressed to a later iteration number, as the
  other party can invalidate C_i by submitting D_i' for some i' > i.
  
  C_i contains operations:
  - One `BEGIN_SPONSORING_FUTURE_RESERVES` operation that specifies reserve
  account V as a sponsor of future reserves.
  - One `CREATE_CLAIMABLE_BALANCE` operation for each trustline that is
  disbursing funds to R.
  - One `END_SPONSORING_FUTURE_RESERVES` operation that confirms reserve
  account V's sponsorship.
  - One or more `SET_OPTIONS` operation adjusting escrow account E's
  thresholds to give I full control of E, and removing R's signers.
  - One or more `SET_OPTIONS` operation adjusting reserve account E's
  thresholds to give R full control of V, and removing I's signers.

- D_i, the _declaration transaction_, declares an intent to execute
the corresponding closing transactions IC_i/RC_i.  D_i has source account E,
sequence number 2i, and `minSeqNum` set to s.  Hence, D_i can execute at any
time, so long as E's sequence number n satisfies s <= n < 2i.  Because C_i
has source account E and sequence number 2i+1, D_i leaves E in a state where
C_i can execute.

  D_i does not require any operations, but since Stellar disallows empty
  transactions, it contains a `BUMP_SEQUENCE` operation with sequence value 0
  as a no-op.

#### Uncooperative Close

TODO: 

#### Cooperative Close

TODO: 

#### Top-up by Initiator

TODO: 

#### Top-up by Responder

TODO: 

#### Withdraw Without Close

TODO: Flesh out with more steps and list operations explicitly.

For R to top-up or withdraw excess funds from the escrow account E, the
participants skip a generation. They set s = 2(i+1), and i = i+2. They then
exchange C_i and D_i (which unlike the update case, can be exchanged in a
single phase of communication because D_i is not yet executable while E's
sequence number is below the new s). Finally, they create a top-up
transaction that atomically adjusts E's balance and uses `BUMP_SEQUENCE` to
increase E's sequence number to s.

To close the channel cooperatively, the parties re-sign C_i with a
`minSeqNum` of s and a `minSeqAge` of 0, then submit this transaction.

### Network Transaction Fees

All transaction fees are paid by the participant submitting the transaction
to the Stellar network.

All transactions defined in the protocol have their fees set to zero.  The
submitter of a transaction wraps the transactions in a fee bump transaction
envelope and provides an appropriate fee, paying the fee themselves.

Credits and debits to escrow account E only ever represent deposits or
withdrawals by I or R, and the sum of all disbursements at close equal the
sum of all deposits minus the sum of all withdrawals.  Network transaction
fees do not change the balance of the channel.

### Reserves

All reserves for new ledger entries created to support the payment channel
are supplied by the participant who will be in control of the ledger entry at
channel close.  Participants should have no impact or dependence on each
other after channel close, and so they must not sponsor ledger entries that
only the other party controls after channel close, either directly or
indirectly through the escrow or reserve accounts.  For example, if the
escrow account was to sponsor the creation of claimable balances at channel
close, participant I would be unable to merge escrow account E until
participant R claimed their claimable balances.

Ledger entries that do not survive channel close, such as signers, are
sponsored by their beneficiary.  A participant should not need to fund
another participants excessive use of signers, participants should pay for
their own key and signing requirements.

Participant I provides reserves for:
- Escrow account E
- Trustlines added to E
- Signers added to E for I
- Signers added to V for I

Participant R provides reserves for:
- Signers added to E for R
- Reserve account V
- Signers added to V for R
- Claimable balances created at close

In the rare event that a network upgrade results in base reserve increasing,
but participant R does not increase the funds in reserve account V to
sufficiently cover the reserve cost, participant I may choose to deposit the
amount of native asset necessary into reserve account V themselves, at some
written-off cost to themselves.

## Security Concerns

The closing transaction, C_i, creates one or more new claimable balance
ledger entries that each require sponsoring.  If the sponsor has insufficient
native asset the closing transactions will fail and consume the sequence
number of the escrow account E, locking the funds.  If that situation occurs
with the most recently agreed upon closing transaction, fair distribution of
the assets depends on both participants signing new closing transactions.

To avoid this situation it is critical that participants lock sufficient
funds up-front to provide the reserve, and that both participants monitor
base reserve changes in the network and respond by adding additional native
asset if required.

## Limitations

This protocol defines the mechanisms of the Stellar network's core protocol
that are used to enforce agreements made by two participants. This protocol
does not define the transport through which the agreements are coordinated,
or the methods through which more than two participants can coordinate and
exchange dependent agreements. These issues are likely to be discussed in
separate proposals.

## Implementations

TODO: Add implementation.

[CAP-21]: https://stellar.org/protocol/cap-21
[CAP-23]: https://stellar.org/protocol/cap-23
[CAP-33]: https://stellar.org/protocol/cap-33
