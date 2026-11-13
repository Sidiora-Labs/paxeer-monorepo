# Paxscan explorer visual system brief (dark and light)

This brief describes the visual system in `apps/explorer`, the Blockscout-based
Next.js frontend using Chakra UI v3. Its theme foundations, component recipes and
page components define the palette, typography and layouts in both colour modes.

The reference style is a contemporary market-data explorer: dark-first, dense, quiet surfaces,
hairline dividers, uppercase micro-labels over every data group, tabular numerals, and a single
prominent search field that reads like a prompt. The light mode is designed with the same care,
not derived by inversion.

## 1. Principles

1. **Density with calm.** Tables are tight (44 px rows), surfaces are flat, shadows are almost
   absent, motion is minimal. Whitespace is spent between groups, not inside them.
2. **One accent.** A single accent hue carries links, active tabs, focus rings and the search
   field. Positive and negative deltas use the semantic green and red only.
3. **Labels above data.** Every data group and every table column is introduced by an uppercase
   11 px label with 0.08 em tracking in the muted text colour.
4. **Numbers are monospaced.** Hashes, addresses, block numbers, amounts and percentages use the
   mono family with `font-variant-numeric: tabular-nums` so columns align.
5. **Data honesty.** A sparkline, price, market cap or change is drawn only from a real series or
   field the backend already returns. Missing data renders as an em dash and a muted label,
   never as a placeholder curve or a zero.
6. **Both modes.** Semantic colour roles provide `_light` and `_dark` values for
   shared surfaces, text, borders, accents and chart colours.

## 2. Colour tokens

`apps/explorer/toolkit/theme/foundations/palette.ts` defines the `LIGHT` and `DARK`
sets. `colors.ts` imports and re-exports them, and
`apps/explorer/toolkit/theme/foundations/semanticTokens.ts` exposes the semantic
roles below.

| Role | Dark | Light |
| --- | --- | --- |
| canvas (page) | `#0B0C10` | `#FFFFFF` |
| surface (cards, tables) | `#111319` | `#F7F8FA` |
| raised (hover rows, menus, popovers) | `#171A21` | `#FFFFFF` |
| hairline (dividers, table rules) | `rgba(255,255,255,0.08)` | `rgba(16,18,24,0.08)` |
| border strong (inputs, focused cards) | `rgba(255,255,255,0.16)` | `rgba(16,18,24,0.16)` |
| text primary | `#F2F4F8` | `#0E1016` |
| text secondary | `#A0A8B8` | `#4B5262` |
| text muted (labels) | `#6B7385` | `#8A90A0` |
| accent | `#4F7DFF` | `#2B4FDA` |
| accent soft (tints, active tab wash) | `rgba(79,125,255,0.14)` | `rgba(43,79,218,0.10)` |
| on accent | `#FFFFFF` | `#FFFFFF` |
| positive | `#2FD27D` | `#0E8F55` |
| positive soft | `rgba(47,210,125,0.14)` | `rgba(14,143,85,0.10)` |
| negative | `#FF5C5C` | `#D1343B` |
| negative soft | `rgba(255,92,92,0.14)` | `rgba(209,52,59,0.10)` |
| warning | `#F5B544` | `#B46A00` |
| header bar | `rgba(11,12,16,0.85)` + 12 px blur | `rgba(255,255,255,0.85)` + 12 px blur |
| focus ring | accent at 2 px, 2 px offset | accent at 2 px, 2 px offset |
| sparkline up / down | positive / negative | positive / negative |
| chart grid | hairline | hairline |
| chart area fill | accent 0.30 to 0 | accent 0.30 to 0 |

The existing `graphStart` / `graphStop`, `success*`, `destructive*`, `warning*`, `tonal`,
`outline`, `dataTableBorder` and `selectedOption` keys map to the roles above through
the compatibility aliases in `palette.ts`.

## 3. Typography

`apps/explorer/toolkit/theme/foundations/typography.ts` defines these families and text styles.

- Body family: the configured body typeface (Inter by default) with the existing fallback stack.
- Heading family: the same body family. Headings are distinguished by weight and size, not by a
  second display face.
- Mono family: `"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, monospace` for hashes,
  addresses, numbers and code.
- Scale (size / line height / weight):
  - display: 40 / 44 / 600, letter-spacing -0.02 em (home hero only)
  - h1: 28 / 32 / 600, -0.01 em (entity titles)
  - h2: 20 / 24 / 600 (section titles)
  - body: 14 / 20 / 400
  - small: 13 / 18 / 400 (table cells)
  - label: 11 / 16 / 600, uppercase, +0.08 em, muted colour (section and column labels)
  - number: 13 / 18 / 500, mono, tabular numerals
  - stat value: 24 / 28 / 600, mono, tabular numerals

## 4. Layout grammar

- One top bar, no vertical sidebar on desktop (the horizontal navigation layout is the only
  layout; the `NEXT_PUBLIC_NAVIGATION_LAYOUT` value `vertical` is accepted and ignored).
- Content column: 1280 px maximum by default. Setting
  `NEXT_PUBLIC_MAX_CONTENT_WIDTH_ENABLED=false` changes the maximum to 10000 px.
  The desktop header uses 24 px side gutters.
- Page = entity header, tabs, then sections. Sections stack with 32 px gaps; inside a section
  the label sits 8 px above its content.
- Stat tiles and chart panels use cards: surface background, hairline border,
  8 px radius, 16 px padding, no shadow.
- Tables sit directly on the canvas with a hairline above the header row and between rows;
  no outer card.
- Radii: 6 px controls, 8 px cards, 999 px pills.

## 5. Components

**Top bar** (56 px, sticky, header bar colour with blur). Left: network logo and name. Centre:
primary groups rendered as text buttons with a chevron: *Explore* (Blocks, Transactions,
Tokens, Accounts, Verified contracts), *Network* (Network stats, Validators, Epochs, Gas
tracker), *Developers* (API docs, Contract verification, Charts). Right: compact search field
(320 px pill, placeholder *Search or ask…*, shortcut hint `⌘K` / `Ctrl K`), colour-mode toggle,
account button. Mobile: logo, search icon, burger; the groups become an accordion sheet.

**Navigation panel** (dropdown under a group): raised surface, hairline border, 8 px radius,
two columns of links, each link a 14 px title with a 12 px muted description line, hover wash
in accent soft.

**Search as prompt.** The hero search on the home page is 640 px wide, 52 px tall, raised
surface, strong border, accent border on focus, placeholder *Search or ask: a transaction,
address, block or token*. Results open in a panel beneath with uppercase group labels
(TRANSACTIONS, ADDRESSES, BLOCKS, TOKENS, APPS) and dense rows. The top-bar field opens the
same panel. `⌘K` / `Ctrl K` focuses the field anywhere.

**Ticker strip** (home, under the top bar): a single horizontally scrolling row of chips, each
chip = token symbol, price in mono and 24 h change coloured positive / negative. Chips are
built from the tokens the API returns with a non-null exchange rate; when none has a rate the
strip is not rendered.

**Stat tile**: label (uppercase), value in the stat value style, an optional hint and
a loading skeleton. `ui/shared/Page/StatTile.tsx` uses an 8 px radius and 16 px padding.

**Table**: header row 36 px with uppercase labels, body rows 44 px, hairline rules, raised
background on hover, first column sticky on narrow screens, numbers right-aligned in mono,
hashes truncated in the middle with a copy affordance on hover. Mobile keeps the existing
list-item fallback, restyled with the same tokens.

**Tabs** (under an entity header): text tabs with a 2 px accent underline on the active tab,
counts in muted text, horizontal scroll on mobile. Tab content starts 24 px below.

**Entity header**: kind label (uppercase, e.g. TRANSACTION, BLOCK, ADDRESS, TOKEN), title (h1),
identifier line in mono with copy and QR affordances, then a row of pill tags (status,
contract, verified, scam warning).

**Section group**: uppercase label, then a key / value grid: two columns on desktop (key 220
px, value flexible), stacked on mobile; rows separated by hairlines; values in body or mono as
appropriate; a hint icon after the key opens the existing tooltip.

**Status pill**: 999 px radius, soft background and strong text in the semantic colour
(positive = success, negative = failed, muted = pending).

**Network history chart**: inline SVG with a 1.5 px line, area fill, axes and a
hover tooltip. `ui/home/indicators/ChainIndicatorChartContainer.tsx` draws a chart
only when a series has at least two points; absent data renders an em dash.

## 6. Page blueprints

**Home.** Ticker strip; hero with the display heading (network name and a one-line
description) and the search-as-prompt field; a stat row of five tiles (Total blocks, Average
block time, Total transactions, Wallet addresses, Gas price) with transaction history
and configured market charts below;
then two dense tables side by side, LATEST BLOCKS and LATEST TRANSACTIONS, each with a *View
all* link in the label row.

**Blocks / Transactions / Accounts lists.** Page title as h1, filter row, table.

**Tokens (markets view).** Columns: #, Token (logo, name, symbol), Price, Change 24 h, Market
cap, Volume 24 h, Holders. Price, change, market cap and volume render only from the fields
the token API returns (exchange rate, circulating market cap, 24 h volume, holders count);
absent values show an em dash. Price, Market cap and Holders have sort controls when
sorting is available. Category chips above the table (All, Fungible, NFT, Bridged)
map to the existing token type filters.

**Transaction detail.** Entity header (TRANSACTION, hash, status pill, method tag); core tabs
*Overview*, *Token transfers*, *Internal*, *Logs*, *State*, *Raw*, with additional
tabs enabled by data and configuration; the Overview is a stack of
labelled groups: SUMMARY (status, block, timestamp, from, to, value, fee), TRANSFERS (table),
BALANCE CHANGES (table with address, change, pre and post balance), DETAILS (gas, nonce,
position, type, input data).

**Block detail.** Entity header (BLOCK, number, finality pill); tabs *Transactions*,
*Withdrawals* (when present), *Details*; BLOCK INFO group with validator, transactions count,
timestamp, hash, parent hash, gas used, gas limit, base fee, burnt fees, size.

**Address detail.** Entity header (ADDRESS or CONTRACT, name tag or address, pills); tabs
*Overview*, *History*, *Transfers*, *Portfolio*, *Analytics*, *Contract* (contracts only),
*More*. Overview = balance tiles and the ADDRESS INFO group; History = transactions and
internal transactions (sub-tabs); Transfers = token transfers; Portfolio = token holdings and
NFTs with the coin balance history chart; Analytics = the existing counters and charts (txs,
transfers, gas used) in stat tiles; More = logs, withdrawals, blocks validated and the other
existing tabs.

**Token detail.** Entity header (TOKEN, name and symbol, type pill); tabs *Markets* (price,
market cap, volume, holders tiles; a price chart only if the stats service exposes a series),
*History* (transfers), *Holders*, *Metadata* (contract, decimals, supply, verified info),
*Inventory* (NFT collections only).

**Network stats.** Hero tiles (transactions per second, active validators, current epoch,
latest block); RECENT BLOCKS mini table; charts grid in the card style.

**Validators / Epochs.** Table with Rank, Validator, Stake, Stake weight, Commission (fields the
validators API provides); Epochs with Started, First block, Last block, Duration.

## 7. Colour-mode rules

- Shared recipes take their palette values from semantic colour tokens.
- The existing colour-mode toggle and the settings colour theme keep working; the stored
  preference wins over the system preference.
- Playwright includes the `dark-color-mode` project. Theme specifications cover
  semantic-token values in both modes.

## 8. Ask assistant

The search panel includes an Ask row backed by `pages/api/assistant.ts` at
`POST /api/assistant`. It accepts a nonempty `question` of at most 2000 characters
and optional string context fields `route`, `entityKind` and `identifier`.

The server reads `EXPLORER_ASSISTANT_BASE_URL`, `EXPLORER_ASSISTANT_MODEL` and
`EXPLORER_ASSISTANT_API_KEY`. The default model is `xai/grok-4.7`; the configured
base URL supplies the `/chat/completions` endpoint. The request has a 30-second
timeout and refuses redirects. A missing base URL returns 503, malformed input
returns 400, and an upstream request error returns 502. Successful responses
contain `answer` and `model`; responses use `Cache-Control: no-store`.
