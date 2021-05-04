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

- R, the _responder_, who joins the payment channel, and receives disbursement
through claimable balances at channel close.

### Observation Period

A payment channel defines an observation period O within which all
participants are guaranteed to be able to observe the blockchain state and
submit transactions in response to changing state.

The participants agree on the period O at channel open.

The participants may agree to change the period O at anytime by following the
[Change the Observation Period](#Change-the-Observation-Period) process.

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

### Constants

The two participants agree on the following constants:

- m, the _transaction set maximum transaction count_, is defined as 2, the
maximum number of transactions that can be signed in any process between the
increments of iteration number i. This constant may be changed in protocol
upgrades.

### Variables

The two participants maintain the following variables during the lifetime of
the channel:

- s, the _starting sequence number_, is initialized to one greater than the
sequence number of escrow account E after E has been created. It the first
available sequence number for iterations to consume.

- i, the _iteration number_ of the payment channel, is initialized to zero.
It is incremented with every off-chain update of the payment channel state,
or on-chain setup, deposit, withdrawal.

- e, the _last executed iteration_, is initialized to zero. It is incremented
with every on-chain setup, deposit, withdrawal.

### Computed Values

The two participants frequently use the following computed values:

- s_i, the _iteration sequence number_, is the sequence number that iteration
i's transaction set starts at. Assuming the history of the payment channel has a
single value for m it is computable as, s+(m*i).

- s_e, the _last executed iteration sequence number_, is the sequence number
that the last agreed-to-be-executed iteration i's transaction set starts at,
where i is e. Assuming the history of the payment channel has a single value for
m it is computable as, s+(m*e).

### Processes

#### Setup

To setup the payment channel:

1. I creates the escrow account E.
2. R creates the reserve account V.
3. Set variable initial states:
   - s to E's sequence number + 1.
   - i to 0.
5. Increment i.
4. I and R build the formation transaction F.
6. I and R follow the [Update Process](#Update-Process), including the step to
increment i, to build, sign, and exchange declaration and closing transactions
that allow the payment channel to be closed with disbursements matching the
initial contributions. This step yields transactions D_i' and C_i' where i' =
i+1.
7. I and R sign and exchange signatures for formation transaction F.
8. I or R submit F.
9. Set e to F's iteration number.

The transactions are constructed as follows:

- C_i', see [Update Process](#Update-Process).

- D_i', see [Update Process](#Update-Process).

- F, the _formation transaction_, deposits I and R's contributions to escrow
account E, R's reserves to reserve account V, and changes escrow account E
and reserve account V to be 2-of-2 multisig accounts. F has source account
E, and sequence number set to s_i.

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
  - One or more `PAYMENT` operations depositing R's reserves to V, for each
  trustline on E that will be used to sponsor claimable balances at
  disbursement.
  
#### Update

To update the payment channel state, the participants:

1. Increment i.
2. Sign and exchange a closing transaction C_i.
3. Sign and exchange a declaration transaction D_i.

The transactions are constructed as follows:

- C_i, the _closing transaction_, disburses funds to R and changes the
signing weights on E such that I unilaterally controls E.  C_i has source
account E, sequence number s_i+1, and a `minSeqAge` of O (the observation
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

- D_i, the _declaration transaction_, declares an intent to execute the
corresponding closing transaction C_i.  D_i has source account E, sequence
number s_i, and `minSeqNum` set to s_e.  Hence, D_i can execute at any time, so
long as E's sequence number n satisfies s_e <= n < s_i.  Because C_i has source
account E and sequence number s_i+1, D_i leaves E in a state where C_i can
execute.

  D_i does not require any operations, but since Stellar disallows empty
  transactions, it contains a `BUMP_SEQUENCE` operation with sequence value 0
  as a no-op.

#### Cooperative Close

Participants can agree to close the channel immediately by modifying and
resigning the most recently signed confirmation transaction. The participants
change the `minSeqAge` to zero.

1. Modify the most recent C_i `minSeqAge` to zero.
2. Resign and exchange the modified confirmation transaction C_i.
3. Submit D_i
4. Submit modified C_i

#### Uncooperative Close

Participants can close the channel at the latest state by submitting the most
recently signed declaration transaction, waiting the observation period O, then
submitting the closing transaction.

1. Submit most recent D_i
2. Wait observation period O
3. Submit C_i

#### Contesting an Uncooperative Close

Participants can attempt to close the channel at a state that is earlier in the
history of the channel than the most recently agreed to state. A participant who
is a malicious actor might attempt to do this if an earlier state benefits them.

The malicious participant can do this by performing the [Uncooperative
Close](#Uncooperative-Close) process with a declaration transaction that is not
the most recently signed declaration transaction.

The other participant can identify that the close process has started at an
earlier state by monitoring changes in escrow account E's sequence. If the other
participant sees the sequence number of escrow account E change to a value that
is not the most recently used s_i, they can use the following process to contest
the close. A participant contests a close by submitting a more recent
declaration transaction and closing the channel at the actual final state.

1. Get E's sequence number n
2. If s_e > n < s_i, go to 3, otherwise go to 1
3. Submit most recent D_i
4. Wait observation period O
5. Submit C_i

#### Add Trustline

Participants can add additional trustlines if they plan to make deposits of new balances.

1. Increment i.
2. I and R build the trustline transaction TA_i.
3. I and R follow the [Update Process](#Update-Process), including the step to
increment i, to build, sign, and exchange declaration and closing transactions
that close the channel in the same state as the most recently agreed state.
This step yields transactions D_i' and C_i' where i' = i+1.
4. I and R sign and exchange signatures for trustline transaction TA_i.
5. I or R submit TA_i.
6. Wait for E's sequence number to be TA_i's.
6. Set e to TA_i's iteration number.

The transactions are constructed as follows:

- C_i', see [Update Process](#Update-Process).

- D_i', see [Update Process](#Update-Process).

- TA_i, the _add trustline transaction_, adds one or more trustlines on escrow
account E, and deposits R's reserves to reserve account V. TA_i has source
account E, and sequence number set to s_i.

  TA_i contains operations:

  - Operations sponsored by I:
    - One `BEGIN_SPONSORING_FUTURE_RESERVES` operation that specifies
    participant I as a sponsor of future reserves.
    - One or more `CHANGE_TRUST` operations adding trustlines to E.
    - One `END_SPONSORING_FUTURE_RESERVES` operation that stops I sponsoring
    future reserves of subsequent operations.
  - One or more `PAYMENT` operations depositing R's reserves to V, for each new
  trustline on E that will be used to sponsor claimable balances at
  disbursement.

#### Remove Trustline

Participants can remove empty trustlines.

1. Increment i.
2. I and R build the trustline transaction TR_i.
3. I and R follow the [Update Process](#Update-Process), including the step to
increment i, to build, sign, and exchange declaration and closing transactions
that close the channel in the same state as the most recently agreed state.
This step yields transactions D_i' and C_i' where i' = i+1.
4. I and R sign and exchange signatures for trustline transaction TR_i.
5. I or R submit TR_i.
6. Wait for E's sequence number to be TR_i's.
6. Set e to TR_i's iteration number.

The transactions are constructed as follows:

- C_i', see [Update Process](#Update-Process).

- D_i', see [Update Process](#Update-Process).

- TR_i, the _add trustline transaction_, removes one or more trustline on escrow
account E, and withdraws R's reserves from reserve account V. TR_i has source
account E, and sequence number set to s_i.

  TR_i contains operations:

  - Operations sponsored by I:
    - One `BEGIN_SPONSORING_FUTURE_RESERVES` operation that specifies
    participant I as a sponsor of future reserves.
    - One or more `CHANGE_TRUST` operations removing trustlines from E.
    - One `END_SPONSORING_FUTURE_RESERVES` operation that stops I sponsoring
    future reserves of subsequent operations.
  - One or more `PAYMENT` operations withdrawing R's reserves from V, for each
  trustline being removed from E that would have been used to sponsor claimable
  balances at disbursement and are no longer required.

#### Deposit by Initiator

Participant I may deposit into the channel without coordination with
participant R, as long as escrow account E already has a trustline for the
asset being deposited.

If participant I wishes to deposit an asset that escrow account E does not hold
a trustline for, the [Add Trustlines](#Add-Trustline) process must be used
first.

#### Deposit by Responder

Participant R may deposit into the channel without coordination with participant
R, as long as escrow account E already has a trustline for the asset being
deposited, and as long as participants R's intent is to make a payment of the
same value to participant I. Any amounts deposited to the payment channel
without coordination will be disbursable to participant I at close.

Participant R must coordinate with participant I to deposit any amount that it
does not intend to immediately pay participant I. The participants use the
following process:

1. Increment i.
2. I and R build the deposit transaction P_i.
3. I and R follow the [Update Process](#Update-Process), including the step to
increment i, to build, sign, and exchange declaration and closing transactions
that close the channel in the same state as the most recently agreed state.
This step yields transactions D_i' and C_i' where i' = i+1.
4. I and R follow the [Update Process](#Update-Process), including the step to
increment i, to build, sign, and exchange declaration and closing transactions
that define how the assets held by the escrow account will be disbursed at close
of the channel such that the deposited amount included in P_i will be disbursed
to participant R. This step yields transactions D_i'' and C_i'' where i'' = i+2.
4. I and R sign and exchange signatures for deposit transaction P_i.
5. I or R submit P_i.
6. Wait for E's sequence number to be P_i's.
6. Set e to P_i's iteration number.

The transactions are constructed as follows:

- C_i', see [Update Process](#Update-Process).

- D_i', see [Update Process](#Update-Process).

- C_i'', see [Update Process](#Update-Process).

- D_i'', see [Update Process](#Update-Process).

- P_i, the _deposit transaction_, makes one or more payments from any Stellar
accounts to escrow account E. P_i has source account E, and sequence number set
to s_i.

  P_i contains operations:

  - One or more `PAYMENT` operations depositing assets into escrow account E.
  - One `BUMP_SEQUENCE` operation bumping the sequence number of escrow account
  E to s_i''.

TODO: This deposit is not safe. If it succeeds the appropriate final state
closure is possible with D_i'' and C_i''. If it fails the existing state is
preserved in D_i' and C_i', however there is now nothing preventing D_i'' and
C_i'' being submitted.

#### Withdraw

TODO: Write this flow out.

#### Change the Observation Period

The participants may agree at anytime to decrease period O by simply using a
smaller value for O in future transaction sets.  The change will only apply
to future transaction sets.  The change does not require submitting a
transaction to the network.

The participants may agree at anytime to increase period O by using a larger
value for O in the next and future transaction sets, or regenerating the most
recent transaction set, then signing and submitting a transaction that bumps the
sequence number of the escrow account to the sequence before the most recent
D_i. The sequence bump ensures only the most recent transaction with the new
period O is valid.

The participants:

1. Increment i.
2. I and R build the bump transaction B_i.
3. I and R follow the [Update Process](#Update-Process), including the step to
increment i, to build, sign, and exchange declaration and closing transactions
that close the channel in the same state as the most recently agreed state.
This step yields transactions D_i' and C_i' where i' = i+1.
4. I and R sign and exchange signatures for deposit transaction B_i.
5. I or R submit B_i.
6. Wait for E's sequence number to be B_i's.
6. Set e to B_i's iteration number.

The transactions are constructed as follows:

- B_i, the _bump transaction_, bumps the sequence number of escrow account E
such that only the most recent transaction set is valid. B has source account E,
sequence number s_i.

  B_i does not require any operations, but since Stellar disallows empty
  transactions, it contains a `BUMP_SEQUENCE` operation with sequence value 0 as
  a no-op.

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

The total reserves required for each participant are:

- Participant I

  - 1 (Escrow Account E)
  - \+ Number of Assets (for Trustlines)
  - \+ 2 x Number of I's Signers

- Participant R

  - 1 (Reserve Account V)
  - \+ Number of Assets (for Claimable Balances)
  - \+ 2 x Number of R's Signers

In the rare event that a network upgrade results in base reserve increasing,
but participant R does not increase the funds in reserve account V to
sufficiently cover the reserve cost, participant I may choose to deposit the
amount of native asset necessary into reserve account V themselves, at some
written-off cost to themselves.

## Security Concerns

### Closing Transaction Failure

The closing transaction, C_i, must never fail.  Under the conditions of the
Stellar Consensus Protocol as it is defined today, and under correct use of
this protocol, there is no known conditions that will cause it to fail.  It
will be either invalid or valid and successful, but not valid and failed.  If
C_i was to be valid and fail it would consume a sequence number and fair
distribution of the assets within the escrow account would require the
cooperation of all participants.

If this protocol is not implemented correctly one condition that can result
in the closing transaction failing is if there is not sufficient native asset
to sponsor the ledger entries created by the transaction.  The closing
transaction creates one or more new claimable balance ledger entries that
each require sponsoring.  If the sponsor has insufficient native asset the
closing transaction will fail.  To avoid this situation it is critical that
participants lock sufficient funds up-front to provide the reserve, and that
both participants monitor base reserve changes in the network and respond by
adding additional native asset if required.

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
