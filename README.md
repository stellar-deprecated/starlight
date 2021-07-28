# Experimental Payment Channels

This repository contains a soup of prototypes, documents, and issues for experiments with Payment Channels as described in [CAP-21] and [CAP-40].

The code and documents in this repository are a work-in-progress and are incomplete. Code and documents will be moved from this repository to other repositories once they are ready for use.

## Index

- [Getting Started](Getting%20Started.md)
- [SEP](specifications/sep-payment-channel-mechanism.md)
- [Examples](examples/)

## Discussions

- Discussions specific to CAP-21 are on the [stellar-dev mailing list](https://groups.google.com/g/stellar-dev/c/N8vzP2Mi89U)
- Discussions specific to CAP-40 are on the [stellar-dev mailing list](https://groups.google.com/g/stellar-dev/c/Wp7gNaJvt40)
- All other discussions are [here](https://github.com/stellar/experimental-payment-channels/discussions)

## Forks

The code in this repository uses forks of some software with partial implementations of [CAP-21] and [CAP-40].

### CAP-21

The forks may not be exactly the same as CAP-21 defines as shortcuts were taken. The account extension was implemented using the existing dangling format and not the `cur` format that CAP-21 proposes. Also, not all general preconditions were implemented. Horizon was updated to expose the preconditions and new accounts state that is helpful to payment channels as a bare minimum, but not all because payment channels primariliy need these capabilities in the transactions and don't necessarily need to see that state in the API. Horizon's ingestion validation was not updated and must be disabled when running these forks. Functionality in Horizon's transaction submission queue was disabled to support `minSeqNum`.

The preconditions available in CAP-21 are listed here, and checked if implemented.

- [x] `timeBounds`
- [ ] `ledgerBounds`
- [x] `minSeqNum`
- [x] `minSeqAge`
- [x] `minSeqLedgerGap`
- [x] `extraSigners`

### CAP-40

The forks have a more complete implementation of CAP-40 because it is a small change.

- [x] ed25519 signed payload signer

### Code

- xdr: https://github.com/leighmcculloch/stellar--stellar-core/tree/cap21/src/xdr
- stellar-core: https://github.com/leighmcculloch/stellar--stellar-core/pull/1
- horizon & Go SDK: https://github.com/stellar/go/pull/3546
- quickstart: https://github.com/leighmcculloch/stellar--docker-stellar-core-horizon/pull/2
- stc: https://github.com/leighmcculloch/xdrpp--stc/pull/1

[CAP-21]: https://stellar.org/protocol/cap-21
[CAP-40]: https://stellar.org/protocol/cap-21
