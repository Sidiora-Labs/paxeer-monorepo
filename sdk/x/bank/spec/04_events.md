<!--
order: 4
-->

# Events

The bank module emits SDK events defined in [types/events.go](../types/events.go)
and [keeper/send.go](../keeper/send.go). Address attributes use the SDK address's
`String()` representation. These are bank accounting events; consumers should
interpret their amounts using the units below.

## Handlers

The [message server](../keeper/msg_server.go) emits `message.module = bank` after
a successful send. The keeper emits the associated balance and transfer events.
For service-routed transactions, [BaseApp](../../../baseapp/baseapp.go) adds
`message.action` using the message type URL. The legacy message path uses `send`
or `multisend` instead.

### MsgSend

| Type | Attribute Key | Attribute Value |
| --- | --- | --- |
| transfer | recipient | Recipient address |
| transfer | sender | Sender address |
| transfer | amount | `msg.Amount.String()` |
| message | module | `bank` |
| message | action | `/cosmos.bank.v1beta1.MsgSend` for service routing |
| message | sender | Sender address |

`SendCoins` also emits `coin_spent` for the sender and `coin_received` for the
recipient, with the same coin amount.

### MsgMultiSend

| Type | Attribute Key | Attribute Value |
| --- | --- | --- |
| transfer | recipient | Output address, one transfer event per output |
| transfer | amount | Output `Coins.String()` |
| message | module | `bank` |
| message | action | `/cosmos.bank.v1beta1.MsgMultiSend` for service routing |
| message | sender | Input address, one sender event per input |

`InputOutputCoins` emits `coin_spent` for each input and `coin_received` for each
output. Its output `transfer` events have no `sender` attribute; pair the inputs
and outputs at the message level.

## Keeper events

Coin amounts are `sdk.Coins.String()` values: integer amounts followed by their
denominations, such as `7uhpx`. A list can contain several denominations.
Wei amounts are `sdk.Int.String()` values: plain decimal integers with no
denomination suffix. `OneUhpxInWei` in the send keeper is `1000000000000`, so
one base-denomination unit is 10^12 wei. Fractional balances belong to the
configured base denomination returned by `sdk.MustGetBaseDenom()`.

### MintCoins

| Type | Attributes | Amount units |
| --- | --- | --- |
| coin_received | `receiver`, `amount` | Coins credited to the minting module account |
| coinbase | `minter`, `amount` | Coins minted into that module account |

`receiver` and `minter` are the module account address. Minting updates supply
as well as the balance; these two events describe the same mint.

### BurnCoins

| Type | Attributes | Amount units |
| --- | --- | --- |
| coin_spent | `spender`, `amount` | Coins debited from the burning module account |
| burn | `burner`, `amount` | Coins burned from that module account |

`spender` and `burner` are the module account address. Burning reduces supply;
these two events describe the same burn.

### AddCoins

`coin_received` has `receiver` and `amount` attributes. The amount is the coins
credited to that address. This event alone does not imply minting.

### SubUnlockedCoins/DelegateCoins

`coin_spent` has `spender` and `amount` attributes. The amount is the coins
removed from the address. `DelegateCoins` also credits the staking module
account through `AddCoins`, producing `coin_received`. Spending alone does not
imply burning.

### SubWei/AddWei

| Method | Type | Attributes | Amount units |
| --- | --- | --- | --- |
| `SubWei` | wei_spent | `spender`, `amount` | Integer wei debited |
| `AddWei` | wei_received | `receiver`, `amount` | Integer wei credited |

These events are emitted for successful nonzero operations. Adding wei can carry
into the whole base-denomination balance; subtracting wei can borrow from it.
The carry or borrow writes balances directly and does not emit an additional
`coin_received` or `coin_spent` event. Treat the wei amount as the complete value
of that operation, including any whole-unit conversion.

### SendCoinsAndWei

This method takes separate whole base-denomination (`amt`) and wei (`wei`)
amounts. It performs `SubWei` and `AddWei`, then emits:

| Type | Attribute Key | Attribute Value |
| --- | --- | --- |
| wei_transfer | recipient | Recipient address |
| wei_transfer | sender | Sender address |
| wei_transfer | amount | The `wei` argument as a decimal integer |

When `amt` is positive, the method also calls `SendCoinsWithoutAccCreation`,
which emits the ordinary coin and transfer events. When both arguments are zero,
it returns without events. A positive `amt` with zero `wei` still produces a
`wei_transfer` with amount `0`, but no `wei_spent` or `wei_received`. The method
uses a cached context and publishes its balance changes and events only when
the complete transfer succeeds.

For example, sending `amt = 2` and `wei = 5` emits wei movements of `5` and coin
movements of `2uhpx` when the base denomination is `uhpx`. The value transferred
is `2000000000005` wei. Do not count `transfer` and its corresponding spent and
received events as separate movements. See the [bank overview](README.md#total-supply)
for supply accounting.

With Alice initially holding `5uhpx` and Bob holding zero, that `amt = 2`,
`wei = 5` example emits the following keeper events in order (address names
stand for their Bech32 strings):

| Type | Attributes |
| --- | --- |
| wei_spent | `spender=Alice`, `amount=5` |
| wei_received | `receiver=Bob`, `amount=5` |
| wei_transfer | `recipient=Bob`, `sender=Alice`, `amount=5` |
| coin_spent | `spender=Alice`, `amount=2uhpx` |
| coin_received | `receiver=Bob`, `amount=2uhpx` |
| transfer | `recipient=Bob`, `sender=Alice`, `amount=2uhpx` |
| message | `sender=Alice` |

Alice finishes with `2uhpx` plus `999999999995` wei, and Bob with `2uhpx` plus
`5` wei. Alice's borrow does not add a separate coin-spent event. Independently,
subtracting `5` wei from `2uhpx` and then adding `5` back emits only
`wei_spent(amount=5)` and `wei_received(amount=5)`, even though both operations
write whole balances. `TestRepairM_669_WeiEvents` in
[repair_m669_test.go](../repair_m669_test.go) checks these exact event streams,
the balances, zero amounts, and atomic failure without published events.
