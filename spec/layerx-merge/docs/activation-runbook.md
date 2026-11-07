# Activation runbook

Plan and store tables regenerated from `node/upgrades.go`
(`knownUpgradePlans`, registered handlers and store-upgrade lists),
`node/app.go:layerxStoreUpgrades` and `storage/common/keys/store_keys.go`.
Bring-up expectations come from `programme/data/goldens/merge-bringup.kvx`.
The node source is authoritative for plan names, handlers and on-disk stores.

## Plans

Apply the five explicit plans recognized by `node/upgrades.go:knownUpgradePlans`
in the order below when pending for the starting chain state. Do not replay
plans already applied. Historical tag-based handlers remain registered separately.
The upgrade info file names the plan and its activation height; the store loader
and handler must use that same plan. The file height must equal the last
committed height plus one at startup.

| Plan | Handler | Stores added | Stores deleted | Precompile gate | Pending rule |
| --- | --- | --- | --- | --- | --- |
| v6.9 | runActivationUpgrade | layerxcustody, layerxanchor, layerxexchange, layerxbridge, launchpad, xweb | none | activationPrecompileGate | done_height == 0 and not activation_modules_present |
| v6.10 | runV610Upgrade | none | none | none | done_height == 0 |
| v6.12.0 | runV6120Upgrade | perps, spot, pricefeed | none | none | done_height == 0 |
| v7.0 | mergeUpgradePlan | sequencer, layerxledger, layerxauth, layerxpay, layerxchains, layerxprograms, layerxagent, lxphost | none | mergePrecompileGate | done_height == 0 |
| v7.0-anchor-store-delete | inline: app.mm.RunMigrations | none | layerxanchor | none | done_height == 0 |

## Merge stores

The merge adds these stores without deleting or renaming any store:

| Store-key constant | On-disk store |
| --- | --- |
| `storekeys.SequencerStoreKey` | `sequencer` |
| `storekeys.LayerXLedgerStoreKey` | `layerxledger` |
| `storekeys.LayerXAuthStoreKey` | `layerxauth` |
| `storekeys.LayerXPayStoreKey` | `layerxpay` |
| `storekeys.LayerXChainsStoreKey` | `layerxchains` |
| `storekeys.LayerXProgramsStoreKey` | `layerxprograms` |
| `storekeys.LayerXAgentStoreKey` | `layerxagent` |
| `storekeys.LayerXHostStoreKey` | `lxphost` |

At `v7.0`, `mergeUpgradePlan` retires anchor writes, returns guarantor bonds
and releases retired custody balances. Historical anchor reads remain served.
Only the later `v7.0-anchor-store-delete` store loader deletes `layerxanchor`;
its registered inline handler runs module migrations. Schedule that plan
after merge acceptance with a separate height, backup and evidence record.
The anchor address stays registered and all its methods then refuse.

## Bring-up checks

These are golden test expectations, not evidence of a live activation.
Run each command from the repository root and check its expected exit.
Refusal tests exit successfully when the handler rejects the bad plan.

### accepted

Test: `TestMergePlanRequiresKeyForFirstSequencedHeight`. Expected test exit: `0`.

```sh
go test ./node -run '^TestMergePlanRequiresKeyForFirstSequencedHeight$' -count=1
```

Expected outcome: the merge plan is accepted.

| Golden input | Value |
| --- | --- |
| `activation_height` | `42` |
| `first_sequenced_height` | `43` |
| `leaders` | `5151515151515151515151515151515151515151515151515151515151515151, 5252525252525252525252525252525252525252525252525252525252525252` |
| `last_seq` | `0` |
| `last_seq_hash` | `0000000000000000000000000000000000000000000000000000000000000000` |
| `receipt_key_first_height` | `43` |
| `receipt_key_last_height` | `43` |

### no-leaders

Test: `TestUpgradesSequencerGenesisAtActivation`. Expected test exit: `0`.

```sh
go test ./node -run '^TestUpgradesSequencerGenesisAtActivation$' -count=1
```

Expected handler error contains: `carries no leader list`.

### lastseq

Test: `TestUpgradesSequencerGenesisAtActivation`. Expected test exit: `0`.

```sh
go test ./node -run '^TestUpgradesSequencerGenesisAtActivation$' -count=1
```

Expected handler error contains: `want 0`.

### no-key

Test: `TestMergePlanRefusesKeyCoveringOnlyActivationHeight`. Expected test exit: `0`.

```sh
go test ./node -run '^TestMergePlanRefusesKeyCoveringOnlyActivationHeight$' -count=1
```

Expected handler error contains: `pins no receipt key for first sequenced height`.

### bad-genesis

Test: `TestUpgradesSequencerGenesisAtActivation`. Expected test exit: `0`.

```sh
go test ./node -run '^TestUpgradesSequencerGenesisAtActivation$' -count=1
```

Expected handler error contains: `sequencer genesis`.

## Refusals

The merge handler refuses invalid sequencer genesis, an empty leader
list, a nonzero lastSeq, or missing receipt-key coverage at H_act + 1.
A refusal stops the block that would apply the plan.
The four golden error fragments are:

| Case | Handler error contains |
| --- | --- |
| no-leaders | `carries no leader list` |
| lastseq | `want 0` |
| no-key | `pins no receipt key for first sequenced height` |
| bad-genesis | `sequencer genesis` |
