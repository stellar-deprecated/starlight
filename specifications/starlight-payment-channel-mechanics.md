## Preamble

```
SEP: ????
Title: Starlight Payment Channel Mechanics
Author: David Mazières <@standford-scs>, Leigh McCulloch <@leighmcculloch>
Track: Standard
Status: Draft
Discussion: https://github.com/stellar/starlight/issues
Created: 2021-04-21
Updated: 2022-03-17
Version: 0.5.1
```

## Summary

This protocol defines the mechanics that two participants use to open and close
a payment channel.

## Dependencies

This protocol is dependent on the not-yet-impemented [CAP-21] and [CAP-40], and
is based on the two-way payment channel protocol defined in those CAPs.

This protocol is also dependent on [CAP-33], that added sponsorship to accounts.

## Motivation

Stellar account holders who frequently transact with each other, but do not
trust each other, are limited to performing all their transactions on-chain
to get the benefits of the network enforcing transaction finality.  The
network is fast, but not as fast as two parties forming an agreement directly
with each other.  For high-frequency transactors it would be beneficial if
there was a simple method on Stellar to allow two parties to hold funds
in an account that is controlled by both parties, where agreements can be
formed and guaranteed to be executable and contested on-chain.  [CAP-21],
[CAP-40] and [CAP-33] introduce new functionality to the Stellar protocol that
make it easier to do this.

## Abstract

This protocol defines the Stellar transactions that two participants use to open
and close a payment channel by using multisig accounts to hold a single asset.

A payment channel has two participants, an initiator I and a responder R.

The protocol assumes some _observation period_, O, such that both parties are
guaranteed to be able to observe the blockchain state and submit transactions
within any period of length O.

The payment channel consists of two 2-of-2 multisig accounts MI and MR,
and a series of transaction sets that contain _declaration_ and _closing_
transactions with MI as their source account, signed by both participants.  The
closing transaction defines the final state of the channel that disburses the
asset from MI to MR and/or from MR to MI such that the final balances of MI and
MR match the amounts belonging to I and R. The closing transaction also returns
control of MI to I and control of MR to R.  Each generation of declaration and
closing transaction sets in the series are an agreement on a new final state for
the channel.

Participants use each iteration of declaration and closing transaction sets to
agree, and continuously re-agree, on a new final states for the channel. They
may choose to modify their agreement based on individual payments to one
another, or on some other basis such as to perform net settlement.

For example, if the channel initial state is $30 of which $10 belongs to I and
$20 belongs to R, the first closing transaction will disburse $10 to I and $20
to R. If I makes a payment of $2 to R, then I and R agree on a new closing
transaction that will disburse $8 to I and $22 to R.

## Specification

### Participants

A payment channel has two participants:

- I, the _initiator_, who proposes the payment channel, and creates the 
multisig account that will be used for sequence numbers.  I creates  
account MI and receives disbursement through regaining control of MI at channel 
close.

- R, the _responder_, who joins the payment channel, and creates the other
multisig account. R creates account MR and receives disbursement 
through regaining control of MR at channel close.

### Observation Period

A payment channel defines an observation period O within which all
participants are guaranteed to be able to observe the blockchain state and
submit transactions in response to changing state.

The participants agree on the period O at channel open.

The observation period is defined both as a duration in time, and a count of
ledgers. The observation period has passed if both the duration and ledger count
have been exceeded. These two properties together are referred to as O
throughout the protocol.

The participants may agree to change the period O at anytime by following the
[Change the Observation Period](#Change-the-Observation-Period) process.

### Multisig Accounts

The payment channel utilizes two Stellar accounts that are both 2-of-2
multisig accounts while the channel is open:

- MI, the _multisig account belonging to I_, that holds the assets that I has
contributed to the channel and that will be distributed to the participants at
channel close according to the final close transactions submitted.  Created by
I.  Jointly controlled by I and R while the channel is open.  Control is
returned to I at close.  Provides sequence numbers for the channel while the
channel is open.

- MR, the _multisig account belonging to R_, that holds the assets that R has
contributed to the channel and that will be distributed to the participants at
channel close according to the final close transactions submitted.  Created by
R.  Jointly controlled by I and R while the channel is open.  Control is
returned to R at close.  Does not provide sequence numbers for the channel in 
anyway.

### Constants

The two participants agree on the following constants:

- m, the _maximum transaction count for an iteration's transaction set_, is
defined as 2, the maximum number of transactions that can be signed in any
process between the increments of iteration number i.

### Variables

The two participants maintain the following variables during the lifetime of
the channel:

- s, the _starting sequence number_, is initialized to one greater than the
sequence number of account MI after MI has been created. It is the first
available sequence number for iterations to consume.

- i, the _iteration number_, is initialized to zero.  It is incremented with
every off-chain update of the payment channel state, or on-chain setup,
withdrawal, etc.

- e, the _executed iteration number_, is initialized to zero. It is updated to
the most recent iteration number i that the participants agree to execute
on-chain, such as a setup, or withdrawal.

### Computed Values

The two participants frequently use the following computed values:

- s_i, the _iteration sequence number_, is the sequence number that iteration
i's transaction set starts at, and is computable as, s+(m*i).

- s_e, the _executed iteration sequence number_, is the sequence number that the
executed iteration e's transaction set starts at, and is computable as, s+(m*e).

### Processes

#### Setup

To setup the payment channel:

1. I creates account MI.
2. R creates account MR.
3. Set variable initial states:
   - s to MI's sequence number + 1.
   - i to 0.
   - e to 0.
4. Increment i.
5. I signs and shares:
   - A open transaction F.
   - A declaration transaction D_i.
   - A closing transaction C_i, that closes the channel without any payment.
6. R signs and submits F.

Participants should check the state of the other participant's account
after the open transaction is submitted to ensure the state is as expected. 
Participants may wish to reduce their exposure to griefing by making deposits of 
initial contributions after the channel is open. See [Security](#Security).

Signatures for F, D_i, and C_i may be shared in a single message.

The transactions are constructed as follows:

- F, the _open transaction_, changes accounts MI and MR to be 2-of-2
multisig accounts. F has source account E, and sequence number set to s.

  F has two signers in its `extraSigners` precondition that ensures that if F is
  authorized, signed, and submitted, that the signatures for D_i and C_i are
  revealed. The signer is specified as:

  - A ed25519 signed payload signer configured with:
    - Payload set to the transaction hash of D_i.
    - Public key set to R's signer.

  - A ed25519 signed payload signer configured with:
    - Payload set to the transaction hash of C_i.
    - Public key set to R's signer.

  F contains operations:

  - Operations sponsored by I for MI:
    - One `BEGIN_SPONSORING_FUTURE_RESERVES` operation that specifies
    participant I as a sponsor of future reserves.
    - One `CHANGE_TRUST` operation configuring trustlines on MI if the asset is 
    not the native asset.
    - One `SET_OPTIONS` operation adjusting account MI's thresholds 
    such that I and R's signers must both sign.
    - One `SET_OPTIONS` operation adding I signers to MI.
    - One `END_SPONSORING_FUTURE_RESERVES` operation that stops I sponsoring
    future reserves of subsequent operations.
  - Operations sponsored by I for MR:
    - One `BEGIN_SPONSORING_FUTURE_RESERVES` operation that specifies
    participant I as a sponsor of future reserves.
    - One `SET_OPTIONS` operation adding I signers to MR.
    - One `END_SPONSORING_FUTURE_RESERVES` operation that stops I sponsoring
    future reserves of subsequent operations.
  - Operations sponsored by R for MR:
    - One `BEGIN_SPONSORING_FUTURE_RESERVES` operation that specifies reserve
    account R as a sponsor of future reserves.
    - One `CHANGE_TRUST` operation configuring trustlines on MR if the asset is 
    not the native asset.
    - One `SET_OPTIONS` operation adjusting account MR's thresholds 
    such that R and I's signers must both sign.
    - One `SET_OPTIONS` operation adding R's signers to MR.
    - One `END_SPONSORING_FUTURE_RESERVES` operation that stops R sponsoring
    future reserves of subsequent operations.
  - Operations sponsored by R for MI:
    - One `BEGIN_SPONSORING_FUTURE_RESERVES` operation that specifies reserve
    account R as a sponsor of future reserves.
    - One `SET_OPTIONS` operations adding R's signers to MI.
    - One `END_SPONSORING_FUTURE_RESERVES` operation that stops R sponsoring
    future reserves of subsequent operations.

  The accounts MI and MR will likely have the necessary trustline
  before the open transaction is built. This means the `CHANGE_TRUST`
  operation will likely be a no-op. The `CHANGE_TRUST` operation must be
  included in the open transaction so that participants are guaranteed the
  trustline is still in the same state after the channel is open. If the 
  operation is not included a participant could intentionally or accidentally 
  remove a trustline between account setup and open causing the 
  presigned closing transaction to become invalid.

  The `CHANGE_TRUST` operations configure the trustlines with the maximum limit,
  which is the maximum value of an `int64`, `0x7FFFFFFFFFFFFFFF`.

- C_i, with no payment, see [Payment](#Payment) process.

- D_i, see [Payment](#Payment) process.

#### Payment

Participants use the payment process to agree, and re-agree, on a new final
state for the channel. Participants will agree on a new final state when making
payments to one another within the channel.

For example, if the channel initial state is $30 of which $10 belongs to I and
$20 belongs to R, the first closing transaction will disburse $10 to I and $20
to R. If I makes a payment of $2 to R, this payment process will involve I and R
agreeing on a new declaration and closing transaction that supersedes all
previous declaration and closing transactions and that will disburse $8 to I and
$22 to R.

To make a payment, participants agree on a new payment channel state. The
participants:

1. Increment i.
2. The payer participant signs and shares signatures for:
   - A declaration transaction D_i.
   - A closing transaction C_i.
3. The payee participant signs and shares signatures D_i and C_i.

Signatures for D_i and C_i may be shared in a single message.

The transactions are constructed as follows:

- C_i, the _closing transaction_, disburses funds from MI to MR and/or from MR
to MI, and changes the signing weights on MI such that I unilaterally controls
MI, and the signing weights on MR such that R unilaterally controls MR.  C_i has
source account MI, sequence number s_i+1, a `minSeqAge` of O (the observation
period time duration), and a `minSeqLedgerGap` of O (the observation period
ledger count).

  The `minSeqAge` and `minSeqLedgerGap` prevents a misbehaving party from
  executing C_i when the channel state has already progressed to a later
  iteration number, as the other party has the period O to invalidate C_i by
  submitting D_i' for some i' > i.
  
  C_i contains operations:
  - Zero or one `PAYMENT` operations that disburses funds from MI to MR, or from
  MR to MI, that may be omitted if the final state at this update does not
  require the movement of funds.
  - One or more `SET_OPTIONS` operation adjusting account MI's 
  thresholds to give I full control of MI, and removing R's signers.
  - One or more `SET_OPTIONS` operation adjusting reserve account MR's
  thresholds to give R full control of MR, and removing I's signers.

- D_i, the _declaration transaction_, declares an intent to execute the
corresponding closing transaction C_i.  D_i has source account MI, sequence
number s_i, and `minSeqNum` set to s_e.  Hence, D_i can execute at any time, so
long as MI's sequence number n satisfies s_e <= n < s_i.  Because C_i has source
account MI and sequence number s_i+1, D_i leaves MI in a state where C_i can
execute.

  D_i has a single signer in its `extraSigners` precondition that ensures that
  if D_i is authorized, signed, and submitted, that the signature for C_i is
  revealed. The signer is specified as:

  - A ed25519 signed payload signer configured with:
    - Payload set to the transaction hash of C_i.
    - Public key set to payee's signer.

  D_i does not require any operations, but since Stellar disallows empty
  transactions, it contains a `BUMP_SEQUENCE` operation with sequence value 0
  as a no-op.

#### Coordinated Close

Participants can agree to close the channel immediately by modifying and
resigning the most recently signed confirmation transaction. The participants
change `minSeqAge` and `minSeqLedgerGap` to zero.

1. Submit most recent D_i
2. Modify the most recent C_i `minSeqAge` and `minSeqLedgerGap` to zero
3. Resign and exchange the modified close transaction C_i
4. Submit modified C_i

If participants choose to coordinate a close before submitting D_i they must
take care that the new C_i's hash is included in the D_i's `extraSigners`,
or keep a copy of the old C_i's signature required to satisfy D_i's 
`extraSigners` precondition.

#### Uncoordinated Close

Participants can close the channel after the observation period O without
coordinating. They do this by submitting the most recently signed declaration
transaction, waiting the observation period O, then submitting the closing
transaction.

1. Submit most recent D_i
2. Wait observation period O
3. Submit C_i

#### Contesting a Close

Participants can contest a close if the close is not the most recent agreed
closing state of the payment channel.

Participants can attempt to close the channel at a state that is earlier in the
history of the channel than the most recently agreed to state. A participant who
is a malicious actor might attempt to do this if an earlier state benefits them.

The malicious participant can do this by performing the [Uncoordinated
Close](#Uncoordinated-Close) process with a declaration transaction that is not
the most recently signed declaration transaction.

The other participant can identify that the close process has started at an
earlier state by monitoring changes in account MI's sequence. If the
other participant sees the sequence number of account MI change to a
value that is not the most recently used s_i, they can use the following process
to contest the close. A participant contests a close by submitting a more recent
declaration transaction and closing the channel at the actual final state. A
more recent declaration transaction may be submitted because it has a higher
sequence number than the declaration transaction that the malicious actor
submitted. The more recent declaration transaction prevents the malicious actor
from submitting the older closing transaction because it has a lower sequence
number making that transaction invalid.

1. Get MI's sequence number n
2. If s_{e+1} >= n < s_i, go to step 3, else go to step 1
3. Submit most recent D_i
4. Wait observation period O
5. Submit C_i

#### Changing the Channel Setup

The payment channel setup can be altered with on-chain transactions after
channel setup. Some of the operations used to alter the channel setup may fail
even if the transactions are valid, while others will always succeed if the
transactions are valid.

Some operations are implemented in a two-step process. Participants agree on a
new closing state at a future iteration by signing C_i and D_i transactions
where i has skipped an iteration that is not yet executable because the D_i's
`minSeqNum` is also set in the future. Participants then sign a transaction to
make the change that only moves the sequence of account MI to satisfy 
the `minSeqNum` of the future D_i.

Operations that can fail and change the balances of the channel have the
following requirements as well:

- The transaction that can fail must have its source account set to an account
that is not account MI.
- The transaction that can fail must contain a `BUMP_SEQUENCE` operation that
bumps account MI's sequence number to a sequence number that makes the
D_i executable.

Operations where failure cannot occur or is of no consequence:

- [Change the Observation Period](#Change-the-Observation-Period)
- [Deposit / Top-up](#Deposit--Top-up)

Operations that can fail and where the additional requirements apply:
- [Withdraw](#Withdraw)

##### Deposit / Top-up

Participants may deposit into the channel without coordination, as long as both
accounts MI and MR already have a trustline for the asset being
deposited.

Participant I deposits or tops-up their balance by using a standard payment
operation to MI.

Participant R deposits or tops-up their balance by using a standard payment
operation to MR.

If participants wish to deposit an asset that accounts MI or MR do not
hold a trustline for, the [Add Trustlines](#Add-Trustline) process must be used
first.

##### Withdraw

Participants must coordinate to withdraw an amount without closing the channel.
The participants use the following process, where W is the participant
withdrawing and X is the participant witnessing the withdrawal:

1. Increment i.
2. Set e' to the value of e.
3. Set e to i.
4. W signs and shares:
   - A withdrawal transaction W_i.
   - A declaration transaction D_i+1.
   - A declaration transaction C_i+1, that closes the channel with disbursement
   matching the most recent agreed state, but reducing W's disbursed amount by
   W's withdrawal amount.
5. X signs and shares W_i, D_i+1, and C_i+1.
6. Increment i.
7. I or R submit W_i.

If the withdrawal transaction W_i fails or is never submitted, the C_i and D_i
are not executable because account MI's sequence number was not bumped 
to s_i.  The participants should take the following steps since the withdrawal 
did not succeed:

8. Set e to the value of e'.

The transactions are constructed as follows:

- W_i, the _withdrawal transaction_, makes one or more payments from the 
account MI and/or MR to any Stellar account. W_i has any source 
account that is not MI, typically the participant proposing the change.

  W_i has two signers in its `extraSigners` precondition that ensures that if
  W_i is authorized, signed, and submitted, that the signatures for D_i+1 and
  C_i+1 are revealed. The signer is specified as:

  - A ed25519 signed payload signer configured with:
    - Payload set to the transaction hash of D_i+1.
    - Public key set to X's signer.

  - A ed25519 signed payload signer configured with:
    - Payload set to the transaction hash of C_i+1.
    - Public key set to X's signer.

  W_i contains operations:

  - One `PAYMENT` operations withdrawing assets from accounts MI 
  and/or MR.
  - One `BUMP_SEQUENCE` operation bumping the sequence number of
  account MI to s_i.

- C_i, see [Payment](#Payment) process.

- D_i, see [Payment](#Payment) process.

##### Change the Observation Period

The participants may agree at anytime to decrease period O by simply using a
smaller value for O in future transaction sets.  The change will only apply to
future transaction sets.  The change does not require submitting a transaction
to the network.

The participants may agree at anytime to increase period O by using a larger
value for O in the next and future transaction sets, or regenerating the most
recent transaction set, then signing and submitting a transaction that bumps the
sequence number of account MI to the sequence before the most recent D_i. The 
sequence bump ensures only the most recent transaction with the new period O is 
valid.

The participant initiating this change is X, and the other participant is Y. The
participants:

1. Increment i.
2. Set e to i.
3. X signs and shares signatures for:
   - A bump transaction B_i.
   - A declaration transaction D_i+1.
   - A closing transaction C_i+1, that closes the channel with disbursement
   matching the most recent agreed state.
4. Y signs and shares signatures for B_i, D_i+1, and C_i+1.
5. Increment i.
6. Y submits B_i.

The transactions are constructed as follows:

- B_i, the _bump transaction_, bumps the sequence number of account MI
such that only the most recent transaction set is valid. B has source account
MI, sequence number s_i.

  B_i has two signers in its `extraSigners` precondition that ensures that if
  B_i is authorized, signed, and submitted, that the signatures for D_i+1 and
  C_i+1 are revealed. The signer is specified as:

  - A ed25519 signed payload signer configured with:
    - Payload set to the transaction hash of D_i.
    - Public key set to Y's signer.

  - A ed25519 signed payload signer configured with:
    - Payload set to the transaction hash of C_i.
    - Public key set to Y's signer.

  B_i does not require any operations, but since Stellar disallows empty
  transactions, it contains a `BUMP_SEQUENCE` operation with sequence value 0 as
  a no-op.

#### Reusing a Channel

After close, accounts MI and MR can be reused for another channel with
the same or different participants. The relevant account creation steps during
[Setup](#Setup) are skipped. All variable values from the closed channel are
discarded and set anew with iteration number i and executed iteration number e
being set to zero.

### Network Transaction Fees

All transaction fees are paid by the participant submitting the transaction to
the Stellar network.

All transactions defined in the protocol with account MI as the source
account have their fees set to zero.  The submitter of a transaction wraps the
transaction in a fee bump transaction envelope and provides an appropriate fee,
paying the fee themselves.

Credits and debits to accounts MI and MR only ever represent deposits 
or withdrawals by I or R, and the sum of all disbursements at close equal the 
sum of all deposits minus the sum of all withdrawals.  Network transaction fees 
do not change the balance of the channel.

### Reserves

All reserves for new ledger entries created to support the payment channel are
supplied by the participant who will be in control of the ledger entry at
channel close.  Participants should have no impact or dependence on each other
after channel close, and so they must not sponsor ledger entries that only the
other party controls after channel close, either directly or indirectly through
the multisig or reserve accounts.

Ledger entries that do not survive channel close, such as signers, are sponsored
by their beneficiary.  Participants pay for their own key and signing
requirements.

Participant I provides reserves for:
- Account MI
- Trustlines added to MI
- Signers added to MI for I
- Signers added to MR for I

Participant R provides reserves for:
- Account MR
- Trustlines added to MR
- Signers added to MR for R
- Signers added to MI for R

The total reserves required for each participant are:

- Participant I

  - 1 (Account MI)
  - \+ Number of Assets (for Trustlines on MI, always 0 or 1)
  - \+ 2 x Number of I's Signers

- Participant R

  - 1 (Account MR)
  - \+ Number of Assets (for Trustlines on MR, always 0 or 1)
  - \+ 2 x Number of R's Signers

Changes in the networks base reserve do not impact the channel.

## Security Concerns

### Sponsoring Ledger Entries

The protocol does not, and must not, create new sponsorship of ledger entries
while the channel is open. Any Stellar operation that creates a ledger entry
depends on sponsorship and the transaction containing the operation may fail
when being applied if the sponsorship cannot be satisfied. When the operation
fails, the transaction containing it fails, consuming the sequence number of the
transaction even though it was not successful. If the sequence number of the
declaration or close transactions are consumed without being successful the
channel may be in a state where participants would need to collaborate honestly
to close the channel.

This constraint is why the protocol does not use claimable balances or
preauthorized transactions, as both features create ledger entries.

### Transaction Signing Order

In many of the processes outlined in the protocol an order is provided to when
transactions should be signed and exchanged. This order is critical to the
protocol. If a participant signs transactions out-of-order they will allow the
other participant to place the channel into a state where disbursement is not
possible without the participants coordinating.

### Closing Transaction Failure

The closing transaction, C_i, must never fail.  Under the conditions of the
Stellar Consensus Protocol as it is defined today, and under correct use of this
protocol, and assuming no changes in the authorized state of the channels
trustlines, there is no known conditions that will cause it to fail.  It will be
either invalid or valid and successful, but not valid and failed.  If C_i was to
be valid and fail it would consume a sequence number and fair distribution of
the assets within the multisig account would require the cooperation of all
participants.

A condition that can result in the closing transaction failing is if the payment
operations between the multisig accounts are changed to pay out to some other
accounts. If those other accounts do not exist, or some attribute of the
accounts do not allow a payment to be received, then the payment operations may
fail and as such a closing transaction containing a payment can fail.

Another condition that can result in the closing transaction failing is if the
payment operations between the multisig accounts would exceed any limits either
account has on making a payment, due to liabilities, or would exceed limits on
the receiving account, such as a trustline limit. Participants must ensure that
the payments they sign for are receivable by the multisig accounts.

### Trustline Authorization

Any trustline on the multisig accounts that have been auth revoked, or could be
auth revoked, could compromise the payment channel's ability to close
successfully.

If the issuer of any auth revocable asset submits an allow trust operation
freezing the amounts in either multisig account, the close transaction may fail 
to process if its payment operation is dependent on amounts frozen.

There is nothing participants can do to prevent this, other than using only auth
immutable assets.

### Trustline Limits

Trustlines on the multisig accounts are defined as always having a maximum 
asset limit. This restriction makes the behavior of the closing transaction as
predictable as possible and simplifies implementations that are designed for
operating on common assets that do not have excessive supply.

Implementations that allow lower asset limits may produce closing transactions
that could fail if the final state makes a payment that would exceed the
destination account's trustline limit.

Implementations that are intended for use with assets that have excessive supply
may also produce closing transactions that could fail if trustline limits would
be exceeded because of excessive deposits.

In both cases a party who is not a participant can deposit an amount into the
multisig accounts to cause the closing transaction's payment to fail. Also a
trustline's buying liabilities could also result in some of the available limit
being consumed causing the closing transaction's payment to fail.

### Clawback

Any trustline on the multisig accounts that has clawback enabled could 
compromise the payment channels ability to close successfully.

If the issuer of a clawback enabled trustline submits a clawback operation for
amounts in either multisig account, the close transaction may fail to process 
if its payment operation is dependent on amounts clawed back.

### Account and Trustline State

Participants can inspect the state of accounts and trustlines before
execution of the open to check the state of the accounts and trustlines are
acceptable, but there is no guarantee that state will remain constant until
after the open transaction is executed.

It is critical to check the state of the other participants account 
and their trustlines after opening the channel because there is no way for 
participants to guarantee that the other participant has not altered its state. 
For example, the other participant could add an additional signer to their 
account. Or, for example, the other participant could intentionally or 
accidentally cause flags on their trustline to be changed, such as the clawback 
enabled flag.

### Account and Trustline Balance

Participants should inspect the state of the multisig accounts and their
trustlines after opening the channel to determine the starting balance of each
participants contribution.

To calculate the balance available for spending, participants should get the
trustline's balance and subtract the trustline's selling liabilities. Selling
liabilities are the sum of all selling offers for the asset and therefore
represent the maximum amount the balance could be reduced if all offers were
consumed.

### Atomic Transaction Signature Disclosure

Processes that require the signing of multiple transactions make use of an
ed25519 signed payload signer proposed in [CAP-40] and the `extraSigners`
precondition proposed in [CAP-21] so that all signatures required from the
receiving participant are disclosed in the first transaction required by the
process.

Participants must observe the network for submissions of the first transaction
so as to collect the signatures in the event that the other participant does not
share the signatures. If a participant fails to do this the other participant
could submit a subset of the transactions required by the process and the
participant will not have any other capability to authorize and submit the
remaining transactions.

It may be observed that it could be possible to create a link between multiple
transactions by using other mechanisms on the Stellar network by using hash
locks, such as the `HASH_X` signer. For example, it has been previously proposed
that a `HASH_X` signer of the close transaction's signature could be included in
the declaration transaction's `extraSigners` requiring the reveal of the close
transaction's signature. However, there is no efficient method to prove that a
`HASH_X` signer is the hash of a valid signature of the close transaction which
introduces some uncertainty for the payer participant. Also, exchanging the hash
would need to occur prior to the agreement signatures being exchanged,
introducing additional messages reducing on-the-wire efficiency.

### Queueing Multiple Payments

This protocol implicitly supports participants sending multiple payments to the
other participant without the other participant confirming each payment
immediately. This introduces some risk to the sending participant if they
queue multiple payments to the receiver without any replies, because the
receiving participant can prevent the channel from being closed for
approximately the observation period multiplied by the number of uncomfirmed
payments. The receiving participant could do this by submitting each payment's
declaration transaction in order and wait just less than the observation period
between each submission. The sending participant would not be in possession of
the most recent authorized declaration transaction and so could not jump ahead
to that most recent payment.

### Presence of a Free Option

Like most multi-party protocols, where multiple parties must authorize a
transaction, there exists a period of time where a free-option may exist. One
party may sign a payment and another party can wait some period of time to
decide if they will authorize it, or if they will fallback to the previously
authorized transaction.

## Limitations

This protocol defines the mechanisms of the Stellar network's core protocol that
are used to enforce agreements made by two participants. This protocol does not
define the transport through which the agreements are coordinated, or the
methods through which more than two participants can coordinate and exchange
dependent agreements. These issues are likely to be discussed in separate
proposals.

## Implementations

Prototype implementation of these mechanics are in the `sdk/state` package of:
https://github.com/stellar/starlight

[CAP-21]: https://stellar.org/protocol/cap-21
[CAP-33]: https://stellar.org/protocol/cap-33
[CAP-40]: https://stellar.org/protocol/cap-40
