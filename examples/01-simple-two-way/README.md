# Example: Simple Two-Way Payment Channel

Use the instructions in the root readme to setup a standalone network and
install `stc` first.

## Accounts

### Ledger Root
```
GBZXN7PIRZGNMHGA7MUUUF4GWPY5AYPV6LY4UV2GL6VJGIQRXFDNMADI
SC5O7VZUXDJ6JBDSZ74DSERXL7W3Y5LTOAMRF7RQRL3TAGAPS7LUVG3L
```

### Participant: Initiator
```
GCOBXOFPZXUEBZK4QS5TKSXN6X2YBM3YHBZW6PQW5KJ73K3GFXL7KJHA
SAGANSXQOLF434C3HZ75IL3QW3WVXL6P7NGHDCYFWCRDRZ6O72YJXAHD
```

### Participant: Responder
```
GD7ZSZ7G2P3N3JOH5C2CIIHNJWJOD6OMT7R542AC4MW2CY5OM47UWUZG
SCSDLYC5NRXLZAH2ELSDKJFY4YSNDKCOBYUAZY753GHTESLVO5JK3JS2
```

### Channel
```
GBK34BGFU2HBRUF53HGPV5ICFYJMDMOAUHNUIPZT5C7FTISQGQRHCRYU
SDMPC36TG7ADLOPDHA43R3DRR4AEGTPWHXCA6UAAPBB5KMQH4ULFXCQT
```

## Transactions

### Setup

Create the participants.
```
cat 1-tx-create-initiator | tee >(stc -) | stc -c - | tee >(stc -txhash -) | stc -post -
cat 2-tx-create-responder | tee >(stc -) | stc -c - | tee >(stc -txhash -) | stc -post -
```

Create the channel. For the sake of simplicity the root will be the source of
this account so that the sequence number of the transaction is predictable,
but ordinarily it should be the initiator.
```
cat 3-tx-create-channel | tee >(stc -) | stc -c - | tee >(stc -txhash -) | stc -post -
```

### Close with I getting 75, R getting 75
Declare a close of the account.
```
date && cat 4-tx-declare-close | tee >(stc -) | stc -c - | tee >(stc -txhash -) | stc -post -
```

Complete the close of the account.
```
date && cat 5-tx-complete-close | tee >(stc -) | stc -c - | tee >(stc -txhash -) | stc -post -
```

### Close with I getting 135, R getting 15
Declare a close of the account.
```
date && cat 6-tx-declare-close | tee >(stc -) | stc -c - | tee >(stc -txhash -) | stc -post -
```

Complete the close of the account.
```
date && cat 7-tx-complete-close | tee >(stc -) | stc -c - | tee >(stc -txhash -) | stc -post -
```
