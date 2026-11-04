# API Documentation

This page documents the explorer's account REST API, mounted at
`/api/account/v2` by [ApiRouter](lib/block_scout_web/routers/api_router.ex)
and [AccountRouter](lib/block_scout_web/routers/account_router.ex). All paths
below are relative to the explorer's origin. The former account v1 examples
are obsolete; use the v2 routes and response shapes here.

The explorer indexes Paxeer mainnet-beta, one BFT network with EVM chain ID
125 (`0x7d`). The kernel is part of the network and proposes blocks; a receipt
is final when its block is committed. Account preferences live in the explorer
database. Sub-chains are tenant domains in the database, not additional node
infrastructure. Node access is documented in
[EthereumJSONRPC](../ethereum_jsonrpc/README.md); the
[Rust client](../../../services/libs/blockscout-client/README.md) documents
public REST client generation and health checks.

- [Session and request requirements](#session-and-request-requirements)
- [BlockScoutWeb.Account.API.V2.UserController](#blockscoutwebaccountapiv2usercontroller)
- [BlockScoutWeb.Account.API.V2.TagsController](#blockscoutwebaccountapiv2tagscontroller)
- [Errors and limits](#errors-and-limits)

## Session and request requirements

Enable account functionality with `ACCOUNT_ENABLED=true` and configure the
account database and authentication provider for the deployment. The
`CheckAccountAPI` plug returns HTTP 404 with
`{"message":"Account functionality is disabled"}` when accounts are disabled.

User operations require a session whose `current_user` has verified email and
an associated identity. Sign in through the deployment's configured account
authentication flow and retain its session cookie. An explorer API key is not
a substitute for this session. Authentication handlers are in
[AuthenticateController](lib/block_scout_web/controllers/account/api/v2/authenticate_controller.ex).

Use `Accept: application/json` and `Content-Type: application/json` for the
JSON request examples. The regular account v2 pipeline fetches the session
and applies `Plug.CSRFProtection`. After signing in, call
`GET /api/account/v2/get_csrf`: it returns HTTP 200, `{"message":"ok"}`, and an
`x-bs-account-csrf` response header. Send that token in the `x-csrf-token`
request header, with the same session cookie, for POST, PUT and DELETE.
Authentication routes with their own pipeline are listed in AccountRouter.

The regular pipeline limits parsed bodies to 100,000 bytes and query strings
to 5,000 bytes. Examples use illustrative hashes and IDs; use records belonging
to the signed-in identity and real indexed transactions or contracts where
validation requires them.

## BlockScoutWeb.Account.API.V2.UserController

[UserController](lib/block_scout_web/controllers/account/api/v2/user_controller.ex)
implements these operations; [UserView](lib/block_scout_web/views/account/api/v2/user_view.ex)
defines their JSON. Successful creates, updates and deletes return HTTP 200.
Delete responses are `{"message":"OK"}`.

### info

`GET /api/account/v2/user/info` returns the signed-in user's profile:

```json
{
  "name": "Example user",
  "email": "user@example.com",
  "avatar": "https://example.com/avatar.png",
  "nickname": "example",
  "address_hash": null
}
```

`address_hash` holds the linked wallet address when present. Profile values
come from the identity and authentication session.

### Private address tags

| Action | Method | Path |
| --- | --- | --- |
| `tags_address` | GET | `/api/account/v2/user/tags/address/` |
| `tags_address` | GET | `/api/account/v2/user/tags/address/:id` |
| `create_tag_address` | POST | `/api/account/v2/user/tags/address/` |
| `update_tag_address` | PUT | `/api/account/v2/user/tags/address/:id` |
| `delete_tag_address` | DELETE | `/api/account/v2/user/tags/address/:id` |

Create with an address hash and label:

```json
{
  "address_hash": "0x1111111111111111111111111111111111111111",
  "name": "Treasury"
}
```

PUT accepts `name` and/or `address_hash`; omitted or null attributes are removed
before the update. A create or update returns `id`, `name`, `address_hash`,
and `address` (the enriched address object from `Helper.address_with_info/4`).
The list response is `{"items":[...],"next_page_params":...}`, not a bare array.
An empty final page is:

```json
{"items": [], "next_page_params": null}
```

Follow the returned `next_page_params` as query parameters on the same list
route until it is null. The GET route with `:id` also invokes the paginated
list action; the ID is a paging cursor, not a single-tag lookup.

### Private transaction tags

| Action | Method | Path |
| --- | --- | --- |
| `tags_transaction` | GET | `/api/account/v2/user/tags/transaction/` |
| `tags_transaction` | GET | `/api/account/v2/user/tags/transaction/:id` |
| `create_tag_transaction` | POST | `/api/account/v2/user/tags/transaction/` |
| `update_tag_transaction` | PUT | `/api/account/v2/user/tags/transaction/:id` |
| `delete_tag_transaction` | DELETE | `/api/account/v2/user/tags/transaction/:id` |

POST requires `transaction_hash` and `name`. The transaction must exist in the
explorer database. PUT accepts either or both fields, omitting null values.
A successful create or update returns this shape:

```json
{
  "id": 64,
  "name": "Payment",
  "transaction_hash": "0x1111111111111111111111111111111111111111111111111111111111111111"
}
```

Lists return `items` and `next_page_params`, using the same cursor convention
as address tags. For example, the last page containing that tag is:

```json
{
  "items": [
    {
      "id": 64,
      "name": "Payment",
      "transaction_hash": "0x1111111111111111111111111111111111111111111111111111111111111111"
    }
  ],
  "next_page_params": null
}
```

Creating a tag for a transaction that has not been indexed returns HTTP 422:

```json
{"errors": {"transaction_hash": ["Transaction does not exist"]}}
```

### Watchlist addresses

| Action | Method | Path |
| --- | --- | --- |
| `watchlist` | GET | `/api/account/v2/user/watchlist` |
| `create_watchlist` | POST | `/api/account/v2/user/watchlist` |
| `update_watchlist` | PUT | `/api/account/v2/user/watchlist/:id` |
| `delete_watchlist` | DELETE | `/api/account/v2/user/watchlist/:id` |

These operations use the identity's first watchlist. `:id` identifies an
address entry in that watchlist. POST and PUT both require the full input:

```json
{
  "address_hash": "0x1111111111111111111111111111111111111111",
  "name": "Treasury",
  "notification_settings": {
    "native": {"incoming": true, "outcoming": false},
    "ERC-20": {"incoming": true, "outcoming": false},
    "ERC-721": {"incoming": true, "outcoming": false},
    "ERC-404": {"incoming": true, "outcoming": false}
  },
  "notification_methods": {"email": true}
}
```

Keep the wire spelling `outcoming`. ERC-1155 settings currently reuse the
ERC-721 values internally and have no separate response entry.

A create or update returns the saved entry with `id`, `name`, `address_hash`,
`address`, `address_balance`, `exchange_rate`, `notification_settings`,
`notification_methods`, `tokens_fiat_value`, `tokens_count`, and
`tokens_overflow`. Balance and market data may be null. GET wraps entries in
`items` with `next_page_params`; forward that cursor to fetch subsequent pages.
For list entries, token statistics cover at most 150 token balances per
address; `tokens_overflow` signals more balances and `tokens_fiat_value` sums
the included balances' fiat values.

Adding the same address twice, including updating an entry to a duplicate
address, returns HTTP 422 with a changeset error such as:

```json
{"errors": {"watchlist_id": ["Address already added to the watch list"]}}
```

### API keys

| Action | Method | Path |
| --- | --- | --- |
| `api_keys` | GET | `/api/account/v2/user/api_keys` |
| `create_api_key` | POST | `/api/account/v2/user/api_keys` |
| `update_api_key` | PUT | `/api/account/v2/user/api_keys/:api_key` |
| `delete_api_key` | DELETE | `/api/account/v2/user/api_keys/:api_key` |

POST and PUT take `{"name":"Dashboard"}`. POST generates a UUID; PUT renames
the key identified by the path. Both return:

```json
{"name": "Dashboard", "api_key": "00000000-0000-4000-8000-000000000001"}
```

GET returns a bare array of these objects. The account allows three keys;
exceeding that limit returns HTTP 422:

```json
{"errors": {"name": ["Max 3 keys per account"]}}
```

### Custom ABIs

| Action | Method | Path |
| --- | --- | --- |
| `custom_abis` | GET | `/api/account/v2/user/custom_abis` |
| `create_custom_abi` | POST | `/api/account/v2/user/custom_abis` |
| `update_custom_abi` | PUT | `/api/account/v2/user/custom_abis/:id` |
| `delete_custom_abi` | DELETE | `/api/account/v2/user/custom_abis/:id` |

POST requires `name`, `contract_address_hash` and `abi`. For example, for an
indexed smart contract:

```json
{
  "name": "Storage",
  "contract_address_hash": "0x1111111111111111111111111111111111111111",
  "abi": [
    {
      "type": "function",
      "name": "get",
      "inputs": [],
      "outputs": [{"name": "", "type": "uint256"}],
      "stateMutability": "view"
    }
  ]
}
```

The ABI is an array of objects. Validation keeps supported function entries;
an ABI with no valid functions is rejected. PUT accepts `name`,
`contract_address_hash` and/or `abi`, omitting null values. Create and update
responses contain `id`, `name`, `contract_address_hash`, `contract_address`
(the enriched address object), and the stored `abi`. GET returns a bare array
of those objects. The account allows 15 custom ABIs; exceeding the limit
returns HTTP 422:

```json
{"errors": {"name": ["Max 15 ABIs per account"]}}
```

## BlockScoutWeb.Account.API.V2.TagsController

[TagsController](lib/block_scout_web/controllers/account/api/v2/tags_controller.ex)
and [TagsView](lib/block_scout_web/views/account/api/v2/tags_view.ex) implement
combined tag lookups. These routes accept hashes, unlike the user tag routes
that accept database IDs. They return HTTP 200 even without a signed-in user;
personal fields are then empty, while public tags can still be returned.
Account functionality must still be enabled.

### tags_address

`GET /api/account/v2/tags/address/:address_hash` returns `watchlist_names`,
`personal_tags` and `common_tags`. With no matching tags:

```json
{"watchlist_names": [], "personal_tags": [], "common_tags": []}
```

Private address labels include `label`, `display_name` and `address_hash`.

### tags_transaction

`GET /api/account/v2/tags/transaction/:transaction_hash` combines tags from the
transaction's addresses and the user's private transaction tag. The current
field is `personal_transaction_tag`:

```json
{
  "watchlist_names": [],
  "personal_transaction_tag": {"label": "Payment"},
  "personal_tags": [],
  "common_tags": []
}
```

`personal_transaction_tag` is null when absent. Unknown or malformed hashes
produce empty tag data rather than a resource-not-found response.

## Errors and limits

[FallbackController](lib/block_scout_web/controllers/account/api/v2/fallback_controller.ex)
handles controller failures. Changeset validation errors use HTTP 422 with
`{"errors":{"field":["message"]}}`; other handled failures use a `message`.

| Status | Condition and response message |
| --- | --- |
| 401 | Missing session: `Unauthorized` |
| 403 | Session exists but cannot authenticate: `Unauthorized`; unverified email uses `Unverified email` and includes `email` |
| 404 | Missing identity: `User not found`; missing watchlist: `Watchlist not found` |
| 404 | Missing update target: `Item not found` |
| 404 | Delete found no owned entry: `Watchlist address not found`, `Tag not found`, `Api key not found`, or `Custom ABI not found` |
| 422 | Changeset errors, including invalid hashes, duplicate entries and account limits |
| 500 | Handled internal failure; the generic `:error` branch returns `Unexpected error` |

Private address and transaction tag names and watchlist names allow 1–35
characters. API key names allow 1–255. `ACCOUNT_PRIVATE_TAGS_LIMIT` defaults to
2,000 and `ACCOUNT_WATCHLIST_ADDRESSES_LIMIT` to 15; these are deployment
settings, while the three-key and 15-ABI limits are module constants. Read
returned validation errors rather than relying on the illustrative IDs or
fixed limits from old test captures.
