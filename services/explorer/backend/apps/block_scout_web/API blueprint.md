FORMAT: 1A
HOST: http://localhost:4000

# Explorer Account API v2

This blueprint describes the explorer's current account HTTP API. `HOST` is
the default local backend origin from the explorer Compose stack. For another
deployment, replace it with the configured explorer backend origin; command
examples use `EXPLORER_ORIGIN`. Node JSON-RPC URLs are not account API hosts.
The former account/v1 examples and their upstream POA host are retired.

Paxeer mainnet-beta is one BFT `paxd` network on EVM chain id `125` (`0x7d`);
its kernel participates in the network and proposes blocks. A receipt is final
when its block is committed. These account APIs store explorer preferences,
not chain accounts or finality evidence. Native identities and indexed chain
APIs are described in the [backend overview](../../README.md#paxeer-x-additions).
The [explorer overview](../../../README.md) covers deployment and node URLs,
and [UPSTREAM.md](../../../../../docs/explorer/UPSTREAM.md) records the import provenance.

Routes come from
[`AccountRouter`](lib/block_scout_web/routers/account_router.ex), mounted by
[`ApiRouter`](lib/block_scout_web/routers/api_router.ex) at `/api/account`.
Contracts below follow the v2
[controllers](lib/block_scout_web/controllers/account/api/v2/),
[views](lib/block_scout_web/views/account/api/v2/) and
[controller tests](test/block_scout_web/controllers/account/api/v2/).

## Availability, sessions and CSRF

`ACCOUNT_ENABLED` must enable account functionality. Otherwise the API returns
HTTP `404` with `{"message":"Account functionality is disabled"}`. Individual
login providers also require their deployment configuration.

User-management routes require a current authenticated session; an explorer
API key does not replace that session. Retain the cookies issued by login,
including cookie updates when a session is renewed. Missing authentication
returns HTTP `401` with `{"message":"Unauthorized"}`. A present but rejected
session returns HTTP `403`; an unverified email produces
`{"message":"Unverified email","email":"..."}`.

After login, `GET /api/account/v2/get_csrf` returns HTTP `200`, body
`{"message":"ok"}` and the token in response header `x-bs-account-csrf`.
Send that token as `x-csrf-token` on session-protected writes, with the same
cookies. Fetch a fresh token after authentication changes the session.
Requests and responses use JSON (`Content-Type: application/json` for bodies).

For example, after the configured login flow has saved cookies in a local
cookie jar:

```sh
EXPLORER_ORIGIN=http://localhost:4000
curl --cookie "$EXPLORER_COOKIE_JAR" --cookie-jar "$EXPLORER_COOKIE_JAR" \
  --include "$EXPLORER_ORIGIN/api/account/v2/get_csrf"
```

Set `EXPLORER_COOKIE_JAR` to that jar's path and copy the returned token into
`EXPLORER_CSRF_TOKEN` for subsequent writes. Tokens and cookies are generated
per session; there are no reusable credentials in this page.

# Group Authentication

## Login routes

All paths in this table are relative to `/api/account/v2`. These routes use
the account pipeline without CSRF protection to establish a session. Successful
wallet, OTP confirmation and Dynamic login return user information and renew
the session cookies.

| Method | Path | Request | Successful response |
| --- | --- | --- | --- |
| GET | `/siwe_message` | Query `address`: EVM address | `{"siwe_message":"..."}` |
| POST | `/authenticate_via_wallet` | JSON `message`, `signature` | User information |
| POST | `/send_otp` | JSON `email` | `{"message":"Success"}` |
| POST | `/confirm_otp` | JSON `email`, `otp` (string) | User information |
| GET | `/authenticate_via_dynamic` | `Authorization: Bearer <provider token>` | User information |

For wallet login, sign the exact `siwe_message` returned by the server and
submit it with the signature. Do not substitute a locally invented message.
The implementation validates the signed message through
[`Explorer.Account.Authentication`](../explorer/lib/explorer/account/authentication.ex).
The OTP and Dynamic routes use their configured providers.

`GET /api/account/v2/authenticate` (query `api_key`) and
`POST /api/account/v2/authenticate` (JSON `api_key`) validate the configured
sensitive-endpoint key **and an existing session**. They return the session
map; they are not login-by-API-key endpoints. They use the session/CSRF
pipeline. `GET /api/account/v2/get_csrf` also requires a session.

Additional session routes are `POST /api/account/v2/address/link`,
`POST /api/account/v2/email/link` and `GET /api/account/v2/email/resend`.
Their provider-dependent payloads and checks are defined by the
[address controller](lib/block_scout_web/controllers/account/api/v2/address_controller.ex)
and [email controller](lib/block_scout_web/controllers/account/api/v2/email_controller.ex).

# Group User information

## User [/api/account/v2/user/info]

### Get user information [GET]

Returns the current explorer user's profile. `UserView` emits these five
fields; provider-dependent values, including `address_hash`, can be null.

+ Response 200 (application/json)

        {
          "name": "Example user",
          "email": "reader@example.com",
          "avatar": null,
          "nickname": "reader",
          "address_hash": null
        }

+ Response 401 (application/json)

        {"message":"Unauthorized"}

# Group Private tags

## Private tag records [/api/account/v2/user/tags/address/]

Paths below are relative to `/api/account/v2/user/tags` and operate on the
current user's private records. IDs identify stored tag records; they are not
address or transaction hashes. Updates accept the fields being changed.

| Method | Path | Request or result |
| --- | --- | --- |
| GET | `/address/` or `/address/{id}` | Paginated address tags (`UserController.tags_address` lists the user's tags) |
| POST | `/address/` | JSON `address_hash`, `name`; returns one address tag |
| PUT | `/address/{id}` | JSON `address_hash` and/or `name`; returns updated tag |
| DELETE | `/address/{id}` | Deletes a tag belonging to this user |
| GET | `/transaction/` or `/transaction/{id}` | Paginated transaction tags (`UserController.tags_transaction` lists the user's tags) |
| POST | `/transaction/` | JSON `transaction_hash`, `name`; returns one transaction tag |
| PUT | `/transaction/{id}` | JSON `transaction_hash` and/or `name`; returns updated tag |
| DELETE | `/transaction/{id}` | Deletes a tag belonging to this user |

List responses contain `items` and `next_page_params`. Follow the returned
pagination parameters until `next_page_params` is null. Address tag objects
contain `id`, `address_hash`, `address` (enriched address information) and `name`;
transaction tag objects contain `id`, `transaction_hash` and `name`.

### Create a private address tag [POST /api/account/v2/user/tags/address/]

+ Request (application/json)

    + Headers

            x-csrf-token: <token from get_csrf>

    + Body

            {"address_hash":"0x3e9ac8f16c92bc4f093357933b5befbf1e16987b","name":"My address"}

+ Response 200 (application/json)

    The created address-tag object, including its assigned `id` and enriched
    `address` object as rendered by `UserView`.

+ Response 422 (application/json)

    Validation failures return `{"errors":{...}}`.

## Tags attached to chain objects

`GET /api/account/v2/tags/address/{address_hash}` returns `personal_tags`,
`watchlist_names` and `common_tags`. The corresponding transaction route,
`GET /api/account/v2/tags/transaction/{transaction_hash}`, also returns
`personal_transaction_tag` (null or an object with `label`). These are
`TagsController` lookups by chain hash, distinct from the user record lists
above. Without a current user, personal tags and watchlist names are empty;
public tags remain available when account functionality is enabled.

# Group Watchlist

## Watchlist addresses [/api/account/v2/user/watchlist]

| Method | Path | Result |
| --- | --- | --- |
| GET | `/api/account/v2/user/watchlist` | `items` and `next_page_params` |
| POST | `/api/account/v2/user/watchlist` | Created watchlist-address object |
| PUT | `/api/account/v2/user/watchlist/{id}` | Updated watchlist-address object |
| DELETE | `/api/account/v2/user/watchlist/{id}` | `{"message":"OK"}` |

POST and PUT require `address_hash`, `name`, `notification_settings` and
`notification_methods.email`. Preserve the wire spelling `outcoming`.
For the Paxeer configuration the notification groups are `native`, `ERC-20`,
`ERC-721` and `ERC-404`. The controller mirrors ERC-721 settings into the
stored ERC-1155 flags; it does not read a separate ERC-1155 group.

### Add a watchlist address [POST]

+ Request (application/json)

        {
          "address_hash": "0x3e9ac8f16c92bc4f093357933b5befbf1e16987b",
          "name": "My address",
          "notification_settings": {
            "native": {"incoming": true, "outcoming": true},
            "ERC-20": {"incoming": true, "outcoming": true},
            "ERC-721": {"incoming": false, "outcoming": false},
            "ERC-404": {"incoming": false, "outcoming": false}
          },
          "notification_methods": {"email": true}
        }

+ Response 200 (application/json)

    A watchlist-address object with `id`, `address`, `address_hash`, `name`,
    `address_balance`, `exchange_rate`, `notification_settings`,
    `notification_methods`, `tokens_fiat_value`, `tokens_count` and
    `tokens_overflow`. Market-dependent values may be null. The list endpoint
    computes token summaries for up to 150 balances per watchlist address;
    `tokens_overflow` indicates more balances.

# Group API keys

## API keys [/api/account/v2/user/api_keys]

| Method | Path | Request | Result |
| --- | --- | --- | --- |
| GET | `/api/account/v2/user/api_keys` | None | Array of key objects |
| POST | `/api/account/v2/user/api_keys` | JSON `name` | Created key object |
| PUT | `/api/account/v2/user/api_keys/{api_key}` | JSON `name` | Updated key object |
| DELETE | `/api/account/v2/user/api_keys/{api_key}` | None | `{"message":"OK"}` |

A key object has `api_key` (the server-assigned UUID string) and `name`.
Management requires the user's session and CSRF token for writes. The path
parameter is the key value, not a numeric ID.

### List API keys [GET]

+ Response 200 (application/json)

        []

# Group Custom ABIs

## Custom ABIs [/api/account/v2/user/custom_abis]

| Method | Path | Request | Result |
| --- | --- | --- | --- |
| GET | `/api/account/v2/user/custom_abis` | None | Array of custom-ABI objects |
| POST | `/api/account/v2/user/custom_abis` | JSON `contract_address_hash`, `name`, `abi` | Created custom-ABI object |
| PUT | `/api/account/v2/user/custom_abis/{id}` | JSON fields to change | Updated custom-ABI object |
| DELETE | `/api/account/v2/user/custom_abis/{id}` | None | `{"message":"OK"}` |

`abi` is a JSON ABI array. Returned objects contain `id`,
`contract_address_hash`, `contract_address` (enriched address information),
`name` and `abi`. IDs and contract metadata come from the deployment's database.

### List custom ABIs [GET]

+ Response 200 (application/json)

        []

# Group Errors

The [account fallback controller](lib/block_scout_web/controllers/account/api/v2/fallback_controller.ex)
defines these responses. CSRF failures are rejected by the pipeline before the
user action. Resource operations are scoped to the authenticated identity.

| HTTP status | JSON response or condition |
| --- | --- |
| 401 | `{"message":"Unauthorized"}` for a missing session; `Wrong API key` for rejected sensitive-endpoint keys; `No Bearer token` for missing Dynamic authorization |
| 403 | Rejected session, including `Unverified email`; `API key not configured on the server` for an unconfigured sensitive-endpoint key |
| 404 | `Account functionality is disabled`, `This endpoint is not configured`, `User not found`, `Watchlist not found`, `Item not found`, or a resource-specific missing-record message |
| 422 | `{"errors":{...}}` for changeset validation; `{"message":"Invalid parameter(s)"}` for format failures |
| 429 | Email-resend throttling includes `message` and `seconds_before_next_resend` |
| 500 | Provider or other errors passed to the fallback, including `{"message":"Unexpected error"}` |

Successful deletes return HTTP `200` with `{"message":"OK"}`. Missing delete
records return HTTP `404` with `Watchlist address not found`, `Tag not found`,
`Api key not found` or `Custom ABI not found` as the `message`, as applicable.
Limits, duplicate names and invalid ABIs are validated by the account schemas;
clients must handle validation errors instead of assuming every request succeeds.
