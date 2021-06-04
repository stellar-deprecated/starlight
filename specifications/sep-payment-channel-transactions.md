## Preamble

```
SEP: ????
Title: Payment Channel Transactions
Author: David Mazières <@standford-scs>, Leigh McCulloch <@leighmcculloch>
Track: Standard
Status: Draft
Discussion: https://github.com/stellar/experimental-payment-channels/issues
Created: 2021-04-21
Updated: 2021-06-03
Version: 0.2.0
```

## Summary

This protocol defines the Stellar transactions that two participants use to
open and close a payment channel.

## Dependencies

This protocol is dependent on the not-yet-impemented [CAP-21], and is based
on the two-way payment channel protocol defined in that CAP's rationale.

This protocol is also dependent on [CAP-33], that added sponsorship to accounts.

## Motivation

Stellar account holders who frequently transact with each other, but do not
trust each other, are limited to performing all their transactions on-chain
to get the benefits of the network enforcing transaction finality.  The
network is fast, but not as fast as two parties forming an agreement directly
with each other.  For high-frequency transactors it would be beneficial if
there was a simple method on Stellar to allow two parties to escrow funds
into an account that is controlled by both parties, where agreements can be
formed and guaranteed to be executable and contested on-chain.  [CAP-21] and
[CAP-33] introduce new functionality to the Stellar protocol that make it
easier to do this.

## Abstract

This protocol defines the Stellar transactions that two participants use to
open and close a payment channel by using escrow accounts to holds funds.

A payment channel has two participants, an initiator I and a responder R.

The protocol assumes some _observation period_, O, such that both parties are
guaranteed to be able to observe the blockchain state and submit transactions
within any period of length O.

The payment channel consists of two 2-of-2 multisig escrow accounts EI and ER,
and a series of transaction sets that contain _declaration_ and _closing_
transactions with EI as their source account, signed by both participants.  The
closing transaction defines the final state of the channel that disburses assets
from EI to ER and/or from ER to EI such that the final balances of EI and ER
match the amounts belonging to I and R. The closing transaction also returns
control of EI to I and control of ER to R.  Each generation of declaration and
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

- I, the _initiator_, who proposes the payment channel, and creates the escrow
account that will be used for sequence numbers.  I creates escrow account EI and
receives disbursement through regaining control of EI at channel close.

- R, the _responder_, who joins the payment channel, and creates the other
escrow account. R creates escrow account ER and receives disbursement through
regaining control of ER at channel close.

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

### Escrow Accounts

The payment channel utilizes two Stellar accounts that are both 2-of-2
multisig accounts while the channel is open:

- EI, the _escrow account belonging to I_, that holds the assets that I has
contributed to the channel and that will be distributed to the participants at
channel close according to the final close transactions submitted.  Created by
I.  Jointly controlled by I and R while the channel is open.  Control is
returned to I at close.  Provides sequence numbers for the channel while the
channel is open.

- ER, the _escrow account belonging to R_, that holds the assets that R has
contributed to the channel and that will be distributed to the participants at
channel close according to the final close transactions submitted.  Created by
R.  Jointly controlled by I and R while the channel is open.  Control is
returned to R at close.  Does not provide sequence numbers for the channel in anyway.

### Variables

The two participants maintain the following variables during the lifetime of
the channel:

- s, the _starting sequence number_, is initialized to one greater than the
sequence number of escrow account EI after EI has been created. It is the first
available sequence number for iterations to consume.

- i, the _iteration number_, is initialized to zero.  It is incremented with
every off-chain update of the payment channel state, or on-chain setup,
withdrawal, etc.

- e, the _executed iteration number_, is initialized to zero. It is updated to
the most recent iteration number i that the participants agree to execute
on-chain, such as a setup, or withdrawal.

### Computed Values

The two participants frequently use the following computed values:

- n, the _number of assets the channel supports_, is the count of assets the channel supports, including the native asset. It has a minimum value of 1.

- m, the _maximum transaction count for an iteration's transaction set_, is
defined as the n + 2, and is the maximum number of transactions that can be
signed in any process between the increments of iteration number i.

- s_i, the _iteration sequence number_, is the sequence number that iteration
i's transaction set starts at. Assuming the history of the payment channel has a
single value for m it is computable as, s+(m*i).

- s_e, the _executed iteration sequence number_, is the sequence number that the
executed iteration e's transaction set starts at. Assuming the history of the
payment channel has a single value for m it is computable as, s+(m*e).

### Processes

#### Setup

To setup the payment channel:

1. I creates and deposits initial contribution to escrow account EI.
2. R creates and deposits initial contribution to escrow account ER.
3. Set variable initial states:
   - s to EI's sequence number + 1.
   - i to 0.
   - e to 0.
4. Increment i.
5. Sign and exchange a closing transaction C_i.
6. Sign and exchange a declaration transaction D_i.
7. I and R sign and exchange the formation transaction F.
8. I or R submit F.

Participants should defer deposits of initial contributions till after formation
for channels that will hold trustlines to issuers that are not auth immutable,
and could be clawback enabled. See [Security](#Security).

It is important that F is signed after C_i and D_i because F will make the
accounts EI and ER 2-of-2 multisig. Without C_i and D_i, I and R would not be
able to close the channel, or regain control of the accounts and the assets
within, without coordinating with each other.

The transactions are constructed as follows:

- F, the _formation transaction_, changes escrow accounts EI and ER to be 2-of-2
multisig accounts. F has source account E, and sequence number set to s.

  F contains operations:

  - Operations sponsored by I:
    - One `BEGIN_SPONSORING_FUTURE_RESERVES` operation that specifies
    participant I as a sponsor of future reserves.
    - One or more `CHANGE_TRUST` operations configuring trustlines on EI.
    - One `SET_OPTIONS` operation adjusting escrow account EI's thresholds such
    that I and R's signers must both sign.
    - One or more `SET_OPTIONS` operations adding I and R's signers to ER.
    - One `END_SPONSORING_FUTURE_RESERVES` operation that stops I sponsoring
    future reserves of subsequent operations.
  - Operations sponsored by R:
    - One `BEGIN_SPONSORING_FUTURE_RESERVES` operation that specifies reserve
    account R as a sponsor of future reserves.
    - One or more `CHANGE_TRUST` operations configuring trustlines on ER.
    - One `SET_OPTIONS` operations adjusting escrow account ER's thresholds such
    that R and I's signers must both sign.
    - One or more `SET_OPTIONS` operations adding I and R's signers to EI.
    - One `END_SPONSORING_FUTURE_RESERVES` operation that stops R sponsoring
    future reserves of subsequent operations.
  
  The escrow accounts EI and ER will likely have all the necessary trustlines
  before the formation transaction is built. This means the `CHANGE_TRUST`
  operations will likely be no-ops. The `CHANGE_TRUST` operations must be
  included in the formation transaction so that participants are guaranteed the
  trustlines are still in the same state after formation. If the operations are
  not included a participant could intentionally or accidentally remove a
  trustline between escrow account setup and formation causing the presigned
  closing transaction to become invalid.

- C_i, see [Update](#Update) process.

- D_i, see [Update](#Update) process.

#### Update

Participants use the update process to agree, and re-agree, on a new final state
for the channel. Participants will agree on a new final state when making
payments to one another within the channel.

For example, if the channel initial state is $30 of which $10 belongs to I and
$20 belongs to R, the first closing transaction will disburse $10 to I and $20
to R. If I makes a payment of $2 to R, this update process will involve I and R
agreeing on new declaration, payment, and closing transactions that supersede
all previous declaration, payment, and closing transaction and that will
disburse $8 to I and $22 to R.

To update the payment channel state, the participants:

1. Increment i.
2. Sign and exchange a closing transaction C_i.
3. Sign and exchange zero or more payment transactions P_{i,a}.
4. Sign and exchange a declaration transaction D_i.

It is important that D_i is signed after C_i and P_i because D_i will invalidate
any previously signed C_i and P_i. If I and R signed and exchanged D_i first
either party could prevent the channel from closing without coordination by
submitting D_i and refusing to sign C_i.  The participants would not be able to
close the channel, or regain control of the accounts, and the assets within
without coordinating with each other.

The transactions are constructed as follows:

- C_i, the _closing transaction_, changes the signing weights on EI such that I
unilaterally controls EI, and the signing weights on ER such that R unilaterally
controls ER.  C_i has source account EI, sequence number s_i+1+p, where p is the
number of payments in this transaction set.

  C_i can only execute after all P_i payment transactions have executed, and is
  therefore prevented from executing until O has passed.

  If the number of payments in this transaction set is zero, C_i must be
  assigned a `minSeqAge` and `minSeqLedgerGap` that the first payment would be
  assigned.

  C_i contains operations:
  - One or more `SET_OPTIONS` operation adjusting escrow account EI's thresholds
  to give I full control of EI, and removing R's signers.
  - One or more `SET_OPTIONS` operation adjusting reserve account ER's
  thresholds to give R full control of ER, and removing I's signers.

- P_{i,a}, the _payment transaction_, disburses a single asset a from EI to ER
or from ER to EI. P_{i,a} has source account EI, sequence number s_i+1+a, where
a is the zero index of the asset where each asset in the channel is assigned an
index. P_{i,0}, the first payment, is assigned a `minSeqAge` of O (the
observation period time duration), and a `minSeqLedgerGap` of O (the observation
period ledger count).

  The `minSeqAge` and `minSeqLedgerGap` prevents a misbehaving party from
  executing the first payment when the channel state has already progressed to a
  later iteration number, as the other party has the period O to invalidate P_i
  by submitting D_i' for some i' > i. Subsequent payments and the closing
  transaction cannot be executed until first payment is executed.

  P_i contains operations:
  - One `PAYMENT` operation for the asset being disbursed from EI to ER, or from
  ER to EI.

- D_i, the _declaration transaction_, declares an intent to execute the
corresponding closing transaction C_i.  D_i has source account EI, sequence
number s_i, and `minSeqNum` set to s_e.  Hence, D_i can execute at any time, so
long as EI's sequence number n satisfies s_e <= n < s_i.  Because C_i has source
account EI and sequence number s_i+1, D_i leaves EI in a state where C_i can
execute.

  D_i does not require any operations, but since Stellar disallows empty
  transactions, it contains a `BUMP_SEQUENCE` operation with sequence value 0
  as a no-op.

#### Coordinated Close

Participants can agree to close the channel immediately by modifying and
resigning the most recently signed confirmation transaction. The participants
change `minSeqAge` and `minSeqLedgerGap` to zero.

1. Submit most recent D_i
2. Modify the most recent P_i `minSeqAge` and `minSeqLedgerGap` to zero
3. Resign and exchange the modified P_i
4. Submit modified P_i
5. Modify the most recent C_i `minSeqAge` and `minSeqLedgerGap` to zero
6. Resign and exchange the modified C_i
7. Submit modified C_i

#### Uncoordinated Close

Participants can close the channel after the observation period O without
coordinating. They do this by submitting the most recently signed declaration
transaction, waiting the observation period O, then submitting the closing
transaction.

1. Submit most recent D_i
2. Wait observation period O
3. Submit P_i
4. Wait observation period O
5. Submit C_i

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
earlier state by monitoring changes in escrow account EI's sequence. If the
other participant sees the sequence number of escrow account EI change to a
value that is not the most recently used s_i, they can use the following process
to contest the close. A participant contests a close by submitting a more recent
declaration transaction and closing the channel at the actual final state. A
more recent declaration transaction may be submitted because it has a higher
sequence number than the declaration transaction that the malicious actor
submitted. The more recent declaration transaction prevents the malicious actor
from submitting the older closing transaction because it has a lower sequence
number making that transaction invalid.

1. Get EI's sequence number z
2. If s_{e+1} >= z < s_i, go to step 3, else go to step 1
3. Submit most recent D_i
4. Wait observation period O
5. Submit P_i
6. Wait observation period O
7. Submit C_i

#### Changing the Channel Setup

The payment channel setup can be altered with on-chain transactions after
channel setup. Some of the operations used to alter the channel setup may fail
even if the transactions are valid, while others will always succeed if the
transactions are valid.

Some operations are implemented in a two-step process. Participants agree on a
new closing state at a future iteration by signing C_i and D_i transactions
where i has skipped an iteration that is not yet executable because the D_i's
`minSeqNum` is also set in the future. Participants then sign a transaction to
make the change that only moves the sequence of escrow account EI to satisfy the
`minSeqNum` of the future D_i.

Operations that can fail and change the balances of the channel have the
following requirements as well:

- The transaction that can fail must have its source account set to an account
that is not escrow account EI.
- The transaction that can fail must contain a `BUMP_SEQUENCE` operation that
bumps escrow account EI's sequence number to a sequence number that makes the
D_i executable.

Operations where failure cannot occur or is of no consequence:

- [Change the Observation Period](#Change-the-Observation-Period)
- [Add Trustline](#Add-Trustline)
- [Remove Trustline](#Remove-Trustline)
- [Deposit / Top-up](#Deposit--Top-up)

Operations that can fail and where the additional requirements apply:
- [Withdraw](#Withdraw)

##### Add Trustline

Participants can add additional trustlines if they plan to make deposits of new balances.

1. I and R sign and exchange signatures for trustline transaction TA_i.
2. I or R submit TA_i.

If the add trustline transaction TA_i fails or is never submitted, there is
no consequence to the channel.

The transactions are constructed as follows:

- TA_i, the _add trustline transaction_, adds one or more trustlines on escrow
accounts EI and ER. TA_i has any source account that is not EI, typically the
participant proposing the change.

  TA_i contains operations:

  - Operations sponsored by I:
    - One `BEGIN_SPONSORING_FUTURE_RESERVES` operation that specifies
    participant I as a sponsor of future reserves.
    - One or more `CHANGE_TRUST` operations adding trustlines to EI.
    - One `END_SPONSORING_FUTURE_RESERVES` operation that stops I sponsoring
    future reserves of subsequent operations.
  - Operations sponsored by R:
    - One `BEGIN_SPONSORING_FUTURE_RESERVES` operation that specifies
    participant R as a sponsor of future reserves.
    - One or more `CHANGE_TRUST` operations adding trustlines to ER.
    - One `END_SPONSORING_FUTURE_RESERVES` operation that stops R sponsoring
    future reserves of subsequent operations.

##### Remove Trustline

Participants can remove empty trustlines.

1. I and R sign and exchange signatures for trustline transaction TR_i.
2. I or R submit TR_i.

If the remove trustline transaction TR_i fails or is never submitted, there is
no consequence to the channel.

The transactions are constructed as follows:

- TR_i, the _remove trustline transaction_, removes one or more trustline on
escrow accounts EI and ER. TR_i has any source account that is not EI, typically
the participant proposing the change.

  TR_i contains operations:

  - One or more `CHANGE_TRUST` operations removing trustlines from EI.
  - One or more `CHANGE_TRUST` operations removing trustlines from ER.

##### Deposit / Top-up

Participants may deposit into the channel without coordination, as long as both
escrow accounts EI and ER already have a trustline for the asset being
deposited.

Participant I deposits or tops-up their balance by using a standard payment
operation to EI.

Participant R deposits or tops-up their balance by using a standard payment
operation to ER.

If participants wish to deposit an asset that escrow accounts EI or ER do not
hold a trustline for, the [Add Trustlines](#Add-Trustline) process must be used
first.

##### Withdraw

Participants must coordinate to withdraw an amount without closing the channel.
The participants use the following process, where W is the participant
withdrawing:

1. Increment i.
2. I and R build the withdrawal transaction W_i.
3. Set e' to the value of e.
4. Set e to i.
5. Increment i.
6. Sign and exchange a closing transaction C_i.
7. Sign and exchange a payment transactions P_i, with disbursements matching the
most recent agreed state, but reducing W's disbursed amount by W's withdrawal
amount.
8. Sign and exchange a declaration transaction D_i.
9. I and R sign and exchange signatures for withdrawal transaction W_i.
10. I or R submit W.

If the withdrawal transaction W_i fails or is never submitted, the C_i, P_i and
D_i are not executable because escrow account EI's sequence number was not
bumped to s_i.  The participants should take the following steps since the
withdrawal did not succeed:

10. Set e to the value of e'.

The transactions are constructed as follows:

- W_i, the _withdrawal transaction_, makes one or more payments from the escrow
account EI and/or ER to any Stellar account. W_i has any source account that is
not EI, typically the participant proposing the change.

  W_i contains operations:

  - One or more `PAYMENT` operations withdrawing assets from escrow accounts EI
  and/or ER.
  - One `BUMP_SEQUENCE` operation bumping the sequence number of escrow account
  EI to s_i.
  
- C_i, see [Update](#Update) process.

- D_i, see [Update](#Update) process.

##### Change the Observation Period

The participants may agree at anytime to decrease period O by simply using a
smaller value for O in future transaction sets.  The change will only apply to
future transaction sets.  The change does not require submitting a transaction
to the network.

The participants may agree at anytime to increase period O by using a larger
value for O in the next and future transaction sets, or regenerating the most
recent transaction set, then signing and submitting a transaction that bumps the
sequence number of the escrow account to the sequence before the most recent
D_i. The sequence bump ensures only the most recent transaction with the new
period O is valid.

The participants:

1. Increment i.
2. I and R build the bump transaction B_i.
3. Increment i.
4. Sign and exchange a closing transaction C_i.
5. Sign and exchange a closing transaction P_i, with disbursements matching the
most recent agreed state.
6. Sign and exchange a declaration transaction D_i.
7. I and R sign and exchange signatures for bump transaction B_i.
8. I or R submit B_i.
9. Set e to B_i's iteration number.

The transactions are constructed as follows:

- B_i, the _bump transaction_, bumps the sequence number of escrow account E
such that only the most recent transaction set is valid. B has source account
EI, sequence number s_i.

  B_i does not require any operations, but since Stellar disallows empty
  transactions, it contains a `BUMP_SEQUENCE` operation with sequence value 0 as
  a no-op.

#### Reusing a Channel

After close, escrow accounts EI and ER can be reused for another channel with
the same or different participants. The relevant account creation steps during
[Setup](#Setup) are skipped. All variable values from the closed channel are
discarded and set anew with iteration number i and executed iteration number e
being set to zero.

### Network Transaction Fees

All transaction fees are paid by the participant submitting the transaction to
the Stellar network.

All transactions defined in the protocol with escrow account EI as the source
account have their fees set to zero.  The submitter of a transaction wraps the
transaction in a fee bump transaction envelope and provides an appropriate fee,
paying the fee themselves.

Credits and debits to escrow accounts EI and ER only ever represent deposits or
withdrawals by I or R, and the sum of all disbursements at close equal the sum
of all deposits minus the sum of all withdrawals.  Network transaction fees do
not change the balance of the channel.

### Reserves

All reserves for new ledger entries created to support the payment channel are
supplied by the participant who will be in control of the ledger entry at
channel close.  Participants should have no impact or dependence on each other
after channel close, and so they must not sponsor ledger entries that only the
other party controls after channel close, either directly or indirectly through
the escrow or reserve accounts.

Ledger entries that do not survive channel close, such as signers, are sponsored
by their beneficiary.  Participants pay for their own key and signing
requirements.

Participant I provides reserves for:
- Escrow account EI
- Trustlines added to EI
- Signers added to EI for I
- Signers added to ER for I

Participant R provides reserves for:
- Escrow account ER
- Trustlines added to ER
- Signers added to ER for R
- Signers added to EI for R

The total reserves required for each participant are:

- Participant I

  - 1 (Escrow Account EI)
  - \+ Number of Assets (for Trustlines on EI)
  - \+ 2 x Number of I's Signers

- Participant R

  - 1 (Escrow Account ER)
  - \+ Number of Assets (for Trustlines on ER)
  - \+ 2 x Number of R's Signers

Changes in the networks base reserve do not impact the channel.

## Security Concerns

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
the assets within the escrow account would require the cooperation of all
participants.

A condition that can result in the closing transaction failing is if the payment
operations between the escrow accounts are changed to pay out to some other
accounts. If those other accounts do not exist, or some attribute of the
accounts do not allow a payment to be received, then the payment operations may
fail and as such a closing transaction containing a payment can fail.

Another condition that can result in the closing transaction failing is if the
payment operations between the escrow accounts would exceed any limits either
account has on making a payment, due to liabilities, or would exceed limits on
the receiving account, such as a trustline limit. Participants must ensure that
the payments they sign for are receivable by the escrow accounts.

### Trustline Authorization

Any trustlines on the escrow accounts that have been auth revoked, or could be
auth revoked, could compromise the payment channel's ability to close
successfully.

If the issuer of any auth revocable asset submits an allow trust operation
freezing the amounts in either escrow account, the close transaction may fail to
process if its payment operations are dependent on amounts frozen.

There is nothing participants can do to prevent this, other than using only auth
immutable assets.

### Clawback

Any trustlines on the escrow accounts that have clawback enabled could
compromise the payment channels ability to close successfully.

If the issuer of any clawback enabled trustline submits a clawback operation for
amounts in either escrow account, the payment transaction for that specific
asset may fail to process if its payment operation is dependent on amounts
clawed back. Any other payment transactions for other assets will not be
affected. The closing transaction will be unaffected.

Participants can inspect the state of trustlines before and after formation to
check if either participant has clawback enabled. Checking the state after
formation is critical because there is no way for participants to guarantee
trustline state until after formation has completed because the state can change
prior to formation. For this reason participants should perform their initial
deposit after formation, unless they trust the asset issuer, or unless the asset
is auth immutable.

## Limitations

This protocol defines the mechanisms of the Stellar network's core protocol that
are used to enforce agreements made by two participants. This protocol does not
define the transport through which the agreements are coordinated, or the
methods through which more than two participants can coordinate and exchange
dependent agreements. These issues are likely to be discussed in separate
proposals.

## Implementations

TODO: Add implementation.

[CAP-21]: https://stellar.org/protocol/cap-21
[CAP-23]: https://stellar.org/protocol/cap-23
[CAP-33]: https://stellar.org/protocol/cap-33
