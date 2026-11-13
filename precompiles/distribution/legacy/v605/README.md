# Historical distribution reward queries

The v6.0.5 distribution implementation at `0x0000000000000000000000000000000000001007`
is frozen for historical replay. Its `rewards(address)` ABI says `view`, but a raw
EVM `CALL` with nonzero native value succeeds when the query is valid. The EVM
debits the caller and credits the precompile address; the query does not refund
that payment. Both whole bank units and fractional wei payments are accepted.
An ABI `view` annotation does not enforce nonpayability for a raw call.

Clients should send zero value for reward queries and use `eth_call` or
`STATICCALL` to read them. Do not send a paid transaction to query rewards or
assume that the historical implementation rejects an accidental payment.

The current distribution executor rejects nonzero reward-query value once the
`v7.0` merge plan has been applied and the call height reaches its recorded done
height. The EVM reverts the payment with the failed query. Before that height,
or without an applied plan, the historical acceptance behavior remains. Historical
tracing continues to select the implementation recorded for the requested height;
later activation does not change v6.0.5 replay. No deployment height is fixed here.

[Routing and value tests](repair_m1255_test.go) exercise production version
selection, ABI transport, distribution rewards and bank balances using
in-memory chain stores. Their fixture activation heights are 10, 20 and 30,
independent of any live network.
