# Getting Started

## Running Core and Horizon
To run a standalone network and Horizon instance use the fork of the quickstart docker image:

```
docker build -t stellar/quickstart:cap21 https://github.com/leighmcculloch/stellar--docker-stellar-core-horizon.git#cap21
```

```
docker run --rm -it -p 8000:8000 --name stellar stellar/quickstart:cap21 --standalone
```

The root account of the network will be:
```
GBZXN7PIRZGNMHGA7MUUUF4GWPY5AYPV6LY4UV2GL6VJGIQRXFDNMADI
SC5O7VZUXDJ6JBDSZ74DSERXL7W3Y5LTOAMRF7RQRL3TAGAPS7LUVG3L
```

## Building Transactions

Use the fork of `stc` (if `which stc` returns something, you will need to uninstall that version first):

```
git clone -b cap21 https://github.com/leighmcculloch/xdrpp--stc stc
cd stc
make depend
make
make install
export STCNET=standalone
```

Build transactions as you normally would, but for the standalone network.
```
stc -edit tx
stc -sign tx | stc -post -
```
