/*
Package state contains a state machine, contained in the Channel type, for
managing a Starlight payment channel.

The Channel type once constructed contains functions for three categories of
operations:
- Open: Opening the payment channel.
- Payment: Making a payment from either participant to the other participant.
- Close: Coordinating an immediate close the payment channel.

The Channel type also provides functions for ingesting data from the network
into the state machine. This is necessary to progress the state of the channel
through states that are based on network activity, such as open and close.

The Open, Payment, and Close operations are broken up into three steps:
- Propose: Called by the payer to create the agreement.
- Confirm: Called by the payee to confirm the agreement.
- Finalize*: Called by the payer to finalize the agreement with the payees
signatures.

	+-----------+      +-----------+
	|   Payer   |      |   Payee   |
	+-----+-----+      +-----+-----+
	      |                  |
	   Propose               |
	      +----------------->+
	      |               Confirm
	      +<-----------------+
	  Finalize*              |
	      |                  |

* Note that the Open and Close processes do not have a Finalize operation, and the
Confirm is used in its place at this time. A Finalize operation is likely to be
added in the future.

None of the primitives in this package are threadsafe and synchronization
must be provided by the caller if the package is used in a concurrent
context.
*/
package state
