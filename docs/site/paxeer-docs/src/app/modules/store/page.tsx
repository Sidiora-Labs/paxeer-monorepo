import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/modules/store')

export default function Store() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Store Module</h1>
        <p className="page-description">
          A single cached-context helper in <code>modules/store/</code> — not a registered chain module.
        </p>
      </div>

      <div className="source-note">
        <strong>Source:</strong> <code>Paxeer-X-Network/modules/store/handler.go</code>
      </div>

      <h2>Overview</h2>

      <p>
        <code>modules/store/</code> contains exactly one file, <code>handler.go</code> (11 lines), exporting one function. It has no <code>module.go</code>, keeper, types, genesis, messages, or queries, and it is not registered in <code>node/app.go</code>. <code>modules/README.md</code> lists it only as &quot;module-level store integration helpers&quot;.
      </p>

      <h2>GetCachedContext</h2>

      <pre><code>{`package store

import (
    sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func GetCachedContext(ctx sdk.Context) (sdk.Context, sdk.CacheMultiStore) {
    ms := ctx.MultiStore()
    msCache := ms.CacheMultiStore()
    return ctx.WithMultiStore(msCache), msCache
}`}</code></pre>

      <p>
        <code>GetCachedContext</code> wraps the context&apos;s multistore in a <code>CacheMultiStore</code> and returns both the derived context and the cache. Writes made through the derived context stay in the cache until the caller writes the cache through; discarding the cache discards the writes. This is the standard SDK pattern for speculative execution.
      </p>

      <h2>What it is not</h2>

      <ul>
        <li>No key-formatting or prefix constants, iterator utilities, or codec helpers are defined here. Modules define their own prefixes and call the SDK <code>KVStore</code> and codec directly.</li>
        <li>No state-migration utilities or test helpers (in-memory stores, mock contexts, snapshots) live in this package.</li>
        <li>It is not a layer over <Link href="/storage">PaxDB storage</Link>; module keepers access state through the SDK multistore without going through this package.</li>
      </ul>

      <h2>Next Steps</h2>

      <ul>
        <li><Link href="/storage">Understand PaxDB storage architecture</Link></li>
        <li><Link href="/modules">Review other Paxeer modules</Link></li>
      </ul>

      <div className="prev-next">
        <Link href="/modules/oracle">
          <div className="prev-next-label">Previous</div>
          <div className="prev-next-title">Oracle Module</div>
        </Link>
        <Link href="/modules/tokenfactory">
          <div className="prev-next-label">Next</div>
          <div className="prev-next-title">Token Factory Module</div>
        </Link>
      </div>
    </DocsLayout>
  )
}
