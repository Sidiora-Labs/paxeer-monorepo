# User profile and wallet components

The header chooses an account UI or wallet UI from `config.features.account`
and `config.features.blockchainInteraction`. Accounts can be used without a
connected wallet. These are explorer account and wallet controls for the same
Paxeer network described in the [release notes](../../../RELEASE_NOTES.md).

## Provider selection

[UserProfileDesktop.tsx](./UserProfileDesktop.tsx), used by
[HeaderDesktop](../header/HeaderDesktop.tsx), selects the desktop component.
[HeaderMobile](../header/HeaderMobile.tsx) performs the same dispatch for mobile:

| Account feature | Blockchain interaction | Desktop | Mobile |
| --- | --- | --- | --- |
| Enabled, `authProvider: 'auth0'` | Either | `profile/auth0/UserProfileDesktop` | `profile/auth0/UserProfileMobile` |
| Enabled, `authProvider: 'dynamic'` | Either | `profile/dynamic/UserProfile` | `profile/dynamic/UserProfile` |
| Disabled | Enabled | `wallet/UserWalletDesktop` | `wallet/UserWalletMobile` |
| Disabled | Disabled | No user control | No user control |

Account selection takes priority. An unrecognized account provider returns
`null`; the wallet fallback is reached only when the account feature is disabled.
Dynamic's component is loaded with `ssr: false` in both headers.

## Configuration

The [account feature](../../../configs/app/features/account.ts) is enabled
outside private mode when `NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED=true` and a provider
can be selected:

- `NEXT_PUBLIC_ACCOUNT_AUTH_PROVIDER=dynamic` together with
  `NEXT_PUBLIC_ACCOUNT_DYNAMIC_ENVIRONMENT_ID` selects Dynamic.
- Otherwise, a configured `NEXT_PUBLIC_RE_CAPTCHA_APP_SITE_KEY` selects Auth0.
  Without either provider configuration, the account feature is disabled.

The [blockchain interaction feature](../../../configs/app/features/blockchainInteraction.ts)
is evaluated separately. Outside private mode, it requires complete single-chain
parameters (ID, name, currency and a non-empty RPC URL list) or enabled multichain
configuration. With those parameters, a Dynamic account environment selects the
Dynamic connector; otherwise `NEXT_PUBLIC_WALLET_CONNECT_PROJECT_ID` selects
Reown. With neither connector configured, blockchain interaction is disabled.
[Web3Provider](../../shared/web3/Web3Provider.tsx) selects the matching provider.

For Paxeer, set `NEXT_PUBLIC_NETWORK_ID=125` and supply
`NEXT_PUBLIC_NETWORK_RPC_URL`, for example
`https://api1.mainnet-beta.paxeer.network/`, alongside the other chain parameters.
The RPC input accepts a URL or JSON array of URLs in
[chain.ts](../../../configs/app/chain.ts). The tracked Paxeer presets supply the
chain identity and currency but leave this RPC input unset.

Dynamic's profile can represent an email-only account, but its SDK context still
comes from the [Dynamic web3 provider](../../shared/web3/providers/DynamicProvider.tsx).
Keep the chain and connector configuration available when selecting Dynamic;
the header's account branch alone does not mount that context.

## Account behavior

The Auth0 [desktop](./profile/auth0/UserProfileDesktop.tsx) and
[mobile](./profile/auth0/UserProfileMobile.tsx) components share
[UserProfileContent](./profile/auth0/UserProfileContent.tsx). Their login flow
starts with `select_method` when blockchain interaction is enabled and `email`
otherwise. Account navigation and email controls remain available without wallet
interaction; linked-address and connected-wallet sections are gated by
`config.features.blockchainInteraction.isEnabled`.

The Dynamic [UserProfile](./profile/dynamic/UserProfile.tsx) has three branches:

1. When logged in with `authMode === 'connect-and-sign'`, it shows a selected
   profile button and the Dynamic profile modal, even without a connected wallet.
2. When a wallet is connected without that authenticated state, it shows
   [UserProfilePopover](./profile/dynamic/UserProfilePopover.tsx) with shared
   wallet content and a login action.
3. Otherwise, it shows a Dynamic connect/login button.

The selected [Dynamic profile button](./profile/dynamic/UserProfileButton.tsx)
shows the wallet address or resolved domain when present. Without an address, it
shows the email handle (the part before `@`) or `My profile`. Logging into an
account and connecting a wallet are separate states.

## Shared wallet content

[UserProfileContentWallet](./profile/common/UserProfileContentWallet.tsx) is used
by Auth0 profile content when blockchain interaction is enabled and by the
Dynamic wallet popover. It shows the connected address and wallet settings, or a
Connect action when disconnected.

The wallet-only [desktop](./wallet/UserWalletDesktop.tsx) and
[mobile](./wallet/UserWalletMobile.tsx) components share
[UserWalletMenuContent](./wallet/UserWalletMenuContent.tsx), which provides the
address, wallet settings and disconnect action. They open the wallet connection
UI when no wallet is connected. These components use the connector selected by
blockchain interaction configuration, rather than assuming WalletConnect for
every provider.

The UI uses the local Chakra UI 3 wrappers documented in the
[toolkit README](../../../toolkit/package/README.md).
