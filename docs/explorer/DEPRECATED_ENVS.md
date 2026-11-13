# Explorer configuration inputs

The frontend reads its current environment variables through
[configs/app](../../apps/explorer/configs/app) and validates container inputs with
[the environment schema](../../apps/explorer/deploy/tools/envs-validator/schema.ts).
The [environment reference](./ENVS.md) lists the supported names and feature settings.

Account authentication uses `NEXT_PUBLIC_ACCOUNT_AUTH_PROVIDER` with
`NEXT_PUBLIC_ACCOUNT_DYNAMIC_ENVIRONMENT_ID` for Dynamic or
`NEXT_PUBLIC_RE_CAPTCHA_APP_SITE_KEY` for Auth0. Wallet interaction uses the
configured chain parameters and either Dynamic or `NEXT_PUBLIC_WALLET_CONNECT_PROJECT_ID`.
See [account configuration](../../apps/explorer/configs/app/features/account.ts) and
[blockchain interaction](../../apps/explorer/configs/app/features/blockchainInteraction.ts).

Rollup configuration uses `NEXT_PUBLIC_ROLLUP_TYPE` and
`NEXT_PUBLIC_ROLLUP_PARENT_CHAIN`; Paxeer mainnet-beta uses chain ID `125` and
the node's root-path JSON-RPC URL. See [rollup configuration](../../apps/explorer/configs/app/features/rollup.ts)
and the [Paxeer X settings](./ENVS.md#paxeer-x-network).
