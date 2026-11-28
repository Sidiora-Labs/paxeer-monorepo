# Perps quoter

`perps-quoter` maintains a bounded ladder of post-only limit orders around the
chain's mark price. It uses one dedicated account on chain 125. Run exactly one
quoter for that account; it reconciles and cancels all of the account's orders,
including orders outside the configured market list when stopping.

The default mode is `dry`: it reads the chain and logs the intended cancellations
and placements, and sends no transaction. Set `PERPS_QUOTER_MODE=live` explicitly
to trade. Quoting always uses 1× leverage. It never approves tokens, deposits
collateral, transfers collateral or withdraws funds.

## Configuration

All amounts named `USD` are decimal dollar amounts, with up to 18 decimal
places. Market ids include zero. Durations are whole seconds from 1 to 86400.

| Variable | Default | Meaning |
| --- | --- | --- |
| `PERPS_CHAIN_ID` | required: `125` | Expected chain id, checked against the node before signing. |
| `PERPS_RPC_URLS` | required | Comma-separated HTTP or HTTPS node endpoints. Startup selects an endpoint that answers with the expected chain id. |
| `PERPS_QUOTER_KEY_FILE` | required | Mounted regular file containing one hexadecimal secp256k1 private key, optionally prefixed with `0x`. Permissions must be `0600` or `0400`. Used in dry mode to identify the account too. |
| `PERPS_QUOTER_MARKETS` | `all` | All active markets, or distinct comma-separated unsigned market ids. |
| `PERPS_QUOTER_LEVELS` | `2` | Levels per side, from 1 through 16. |
| `PERPS_QUOTER_SPREAD_BPS` | `10` | First level's distance from the mark, in basis points. |
| `PERPS_QUOTER_STEP_BPS` | `10` | Additional distance per level, in basis points. The outermost distance must be below 10000. |
| `PERPS_QUOTER_LEVEL_USD` | `25` | Maximum notional of each level before lot rounding. |
| `PERPS_QUOTER_REFRESH_BPS` | `5` | Absolute mark movement from the ladder's anchor that renews the ladder. |
| `PERPS_QUOTER_MAX_QUOTE_AGE_SECONDS` | `30` | Age that renews the ladder even without a price move. |
| `PERPS_QUOTER_MAX_POSITION_USD` | `500` | Absolute position cap per market, measured at the current mark. |
| `PERPS_QUOTER_MAX_PRICE_AGE_SECONDS` | `10` | Maximum age of the pricefeed timestamp. The chain must also report the price as fresh. |
| `PERPS_QUOTER_INTERVAL_SECONDS` | `2` | Delay between rounds. |
| `PERPS_QUOTER_REFUSAL_LIMIT` | `3` | Consecutive transaction refusals before the market rests, from 1 through 100. |
| `PERPS_QUOTER_COOLDOWN_SECONDS` | `30` | Rest after consecutive refusals. Cancellation remains available. |
| `PERPS_QUOTER_MAX_TRANSACTIONS_PER_ROUND` | `8` | Maximum transaction attempts across all markets in a round, from 1 through 256. |
| `PERPS_QUOTER_MODE` | `dry` | Exactly `dry` or `live`; other values refuse startup. |
| `PERPS_QUOTER_STOP` | `false` | Exactly `true` or `false`. `true` starts in cleanup mode and exits after checking the remaining orders. |
| `PERPS_QUOTER_HANDOVER` | `false` | Exactly `true` or `false`. A signal leaves resting orders for the next process when true; the stop variable still cancels them. |
| `PORT` | `8081` | Private health listener port. |

Spread, step and movement bounds are positive integers below 10000. USD bounds
must be positive and fit below 2^128 fixed-point units. Invalid configuration
refuses startup. Configuration diagnostics omit the key file and the complete
RPC endpoint, including path credentials. Neither health responses nor refusal
logs contain the key or node credentials.

## Fund the account

The owner supplies native coin for transaction gas and deposits collateral
before starting live quoting. The venue uses USDL with 6 decimals at
`0x85FcD13735F4309833A503EE804ea32395851479`; the perps precompile is
`0x0000000000000000000000000000000000001015`.

Using the quoting account's wallet, approve the intended USDL amount to the
perps precompile, then call `deposit(token, amount)` with that token and its
6-decimal amount. For example, 1000 USDL is `1000000000` token units. Confirm
the deposit and inspect `getAccountSummary(account, token)` and
`getAccount(account)` before enabling live mode. Keep native gas funds available
for cancellation. Funding and withdrawal are owner operations.

Mount the key file outside the image, readable only by its owner, and set the
required variables. Start with `PERPS_QUOTER_MODE=dry` and inspect the logged
`quoter place` and `quoter cancel` actions. Each action identifies market, side,
level, round, client id, price, notional and base size. Logged prices, sizes,
positions and USD notionals use the venue's 18-decimal integer scale.

The shared service image accepts `SERVICE=perps-quoter`. Its image is published
as `ghcr.io/sidiora-labs/perps-quoter` alongside the indexer, gateway, explorer
and portal images. The owner pulls the quoter image by its published digest.
The deployment registry makes it an owner-started private manual process with
zero default replicas and no automatic restart. Start one instance after
funding; keep its health port private. The key is a mounted file, not an
environment value containing the key.

## Bounds and reconciliation

Each market round reads the market's tick and lot, mark and pricefeed timestamp,
open orders, position and account collateral. Bids round down and asks round up
to the tick. Sizes round down to the lot; levels whose resulting notional is
below the supported venue minimum of 10 USD are omitted. Orders use the
canonical `placeOrder` entry point with post-only time in force, a client id and
zero collateral amount. They cannot take liquidity on placement.

Snapshot reads run side by side at one block. The two-request wave limit keeps
concurrency below the ceiling of eight requests. Order details are read for the
market being quoted and for newly seen order ids; the client retains only each
id's market. A closing snapshot follows
only a transaction for that market, including a refused transaction. Otherwise
health reports the last snapshot read, and dry rounds take no closing snapshot.
The `quoter round` log line includes its duration in `seconds`.

Client ids encode market, side, level and round. Reconciliation retains matching
orders, cancels obsolete orders by client id (by order id for an order without a
client id), confirms cancellation, then places missing levels. A filled or
partially filled ladder is renewed around the current mark. Repeating a round
after a refusal preserves the intended client ids. An uncertain submission
retains its signed transaction and nonce until its receipt is resolved.

On the first snapshot of each market after startup, the engine tries once to
keep its resting ladder. It keeps the orders only when all five conditions hold:

1. The market is selected, its price is fresh and it is outside cooldown.
2. At least one order rests; every client id decodes for this market and all
   orders carry the same round.
3. The round's millisecond timestamp is no later than the current round and
   its age is below the quote lifetime.
4. An integer anchor reproduces every resting order exactly once, with no
   missing or extra orders, using the current ladder settings and position cap.
5. The current mark has not moved from that anchor by the refresh threshold.

The engine derives the anchor from the intersection of the bid and ask rounding
intervals, using integer arithmetic and the existing snapshot. A successful
`quoter keep` log identifies the market, original round and order count. The
quote lifetime still starts at that original round. If keeping fails, ordinary
renewal applies; the engine does not retry keeping after a renewal or cooldown.

Each round attempts at most `PERPS_QUOTER_MAX_TRANSACTIONS_PER_ROUND`
transactions across all markets, counting cancellations and placements, including
refusals. The range is 1 through 256; the default is eight. Each transaction waits
for inclusion before the next is submitted, so at most one signed transaction is
outstanding. At the first transaction refused by the bound, the round stops
without reading another market. The next round starts at that market, walks
once around the list in order, and resets to the first market after a walk
finishes without a transaction refused by the bound. The final market receives
its closing snapshot after any transaction; unreached markets retain their last
observed health. Cancel and place logs describe submitted transactions or dry
plans. No replacement is placed in a market until obsolete orders have been
cancelled. Shutdown drains all orders independently of the round budget,
confirming each cancellation before sending the next.

Dry and live quoting also work through a node that does not serve the pending
tag. When the node returns a JSON-RPC error for the pending account count, the
quoter uses the mined count and waits sixty seconds before asking for the tag
again. Its retained signed transaction remains the record of the one transaction
in flight; the next transaction uses the mined count as its nonce after that
receipt is resolved. A transaction sent elsewhere from the same account can
therefore cause a nonce conflict, counted as a refusal. On a node that serves the
pending tag, the quoter still waits while pending and mined counts differ.
Transport failures, timeouts and unreadable responses remain failed chain reads.

The sum of each side's resting base exposure plus the signed current position
must fit the position cap at the current mark. Levels that would breach it are
omitted. If a position passes the cap, orders that would grow it are withdrawn;
the reducing side may remain. Price moves can change the dollar value of an
existing position; the cap bounds new quoting exposure rather than liquidating
that position.

Before every live placement, the account is read again. Available collateral is
bounded by equity less initial margin, idle collateral and withdrawable
collateral. Each new order reserves its full 1× notional at the higher of its
limit and the mark, plus the account's taker fee. Existing reservations and
placements in other markets consume the same account budget. A level that the
account cannot cover is omitted. Dry planning also shares a conservative budget
across markets and does not spend anticipated cancellation proceeds.

A disabled market, a stale price or an unreadable market snapshot permits only
cancellations for that market. No buy can stand at or above the account's lowest
sell. Transaction refusals are logged with a sanitized venue reason and counted
per market. After the refusal limit, the market withdraws its quotes and rests
until the cooldown ends; fresh prices and collateral checks still apply when it
resumes.

## Stop and health

`SIGINT` or `SIGTERM` stops new quoting. With the default
`PERPS_QUOTER_HANDOVER=false`, it starts cancellation with a separate two-minute
cleanup context. With `PERPS_QUOTER_HANDOVER=true`, it reads the account's orders
once and exits without cancelling. The final `quoter stopped` message includes
`handover: true` and `ordersLeft`; remaining orders are a successful handover.
An unreadable book reports `-1` and exits nonzero.

Starting with `PERPS_QUOTER_STOP=true` always follows the cancellation path before
any quoting, even with handover enabled or a simultaneous signal. Use
`PERPS_QUOTER_MODE=live` for a stop invocation that should cancel live orders.
The stop variable is checked before each round; changing a parent shell's
environment does not change an already running process, so signal that process
to stop it.

Cancellation clears every open order of the account, including inactive and
unselected markets, then independently reads the book. The final `quoter stopped`
message reports `ordersLeft`. Successful cancellation requires zero. A failed
read reports `-1`, and unresolved orders or an unreadable book exit nonzero.
Dry mode logs intended cleanup and sends nothing; if live orders already exist,
its cancellation reports their nonzero count and exits nonzero. Keep the node
reachable and the account funded for gas until cancellation finishes.

Only one process may send transactions for an account at a time. Start the next
process after the previous one exits. For a restart that retains quotes, enable
handover on the previous process, signal it and wait for its exit before starting
the next process with the same ladder settings. Handover sends no transactions
after the signal; overlapping senders can still conflict on orders and nonces.

`GET /healthz`, `/healthz/live` and `/healthz/ready` return the mode, last round
time, and per market the actual resting orders, signed position in base units,
last refusal, total and consecutive refusal counts, rest-until time and last
read error. Planned dry orders appear in logs; health shows the actual book.
The response also includes `transactionsLastRound`, the number of live transaction
attempts in the last round (zero in dry mode). Readiness returns 503 before the first completed round, when rounds cease, and
after shutdown. Shutdown also reports the verified remaining order count.
