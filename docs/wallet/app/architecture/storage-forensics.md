# Application storage forensics

The runtime [registry](../../../../apps/wallet/src/platform/storage/registry.ts)
defines browser storage ownership, schemas, sensitivity, retention, quotas,
migrations, lifecycle resets and corruption policies. The records in
[`repositories.ts`](../../../../apps/wallet/src/platform/storage/repositories.ts)
use bounded version-1 envelopes. Swap namespaces retain their own serialized
formats under the same registry.

| Record | Area | Owner | Sensitivity | Retention and reset |
| --- | --- | --- | --- | --- |
| custody-choice | local | custody-choice | security-relevant | Until custody switch, reset, or uninstall; corruption fails closed |
| contacts | local | contacts | private metadata | Until user deletion, reset, or uninstall |
| recent-recipients | local | contacts | private metadata | Five entries; cleared on logout, account removal, reset, or uninstall |
| preferences | local | preferences | preferences | Currency, language, HTTPS custom RPC, developer display controls, and notification preferences; cleared on reset or uninstall |
| currency-rates | local | preferences | public | USD-based rates for at most 32 currency codes, 4 KiB; cleared on reset or uninstall |
| portfolio-filters | local | portfolio | private metadata | Dust preference and bounded token-address list; cleared on reset or uninstall |
| pwa-dismissals | local | pwa | preferences | Install prompt seven days and notification prompt three days |
| notification-state | local | notifications | private metadata | Bounded counters and timestamps; cleared on logout, account removal, reset, or uninstall |
| metadata-cache | local | metadata | public | Caller TTL, 200 entries, 512 KiB total |
| pending-send | session | operations | private metadata | One validated transaction observation, one session, at most two minutes |
| announcement | local | pwa | preferences | Last acknowledged shipped version until reset |
| submitted-transfers | local | send | private metadata | At most 100 entries, 256 KiB; cleared on logout, custody switch, account removal, reset, or uninstall |
| transfer-observations | local | send | private metadata | At most 100 entries of retained transfer evidence, 512 KiB; cleared on logout, custody switch, account removal, reset, or uninstall |

[`swap-records.ts`](../../../../apps/wallet/src/platform/storage/swap-records.ts)
registers two local-storage namespaces, both owned by `swap` and classified as
security-relevant. Each is bounded to 1,000 records and 2 MiB, fails closed on
corruption, and is cleared on reset or uninstall:

| Namespace | Key prefix | Contents |
| --- | --- | --- |
| swap-records | `pax:swap:` | Version-1 token and operation records scoped by network and wallet |
| swap-journal | `paxeer:swap-journal:v1:` | Version-1 intent journals scoped by chain and wallet |

The embedded wallet's Supabase session is managed by
[`IdentitySession`](../../../../apps/wallet/src/wallet/identity.ts), outside the
application record registry.

Malformed optional records are removed and surfaced through the background
status channel. Corrupt custody choice or swap state fails closed.
`resetStorageForLifecycle` deletes only records and namespaces registered for
the requested lifecycle.
