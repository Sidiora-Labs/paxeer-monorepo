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
        <strong>Source:</strong> <code>modules/store/handler.go</code>
      </div>

      <h2>Overview</h2>

      <p>
        <code>modules/store/</code> provides <code>handler.go</code>, which exports the cached-context helper. Keepers and module lifecycle registration live in <code>node/app.go</code>. <code>modules/README.md</code> describes this package as &quot;module-level store integration helpers&quot;.
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

      <h2>Using the cached context</h2>

      <ul>
        <li>Modules define their own prefixes and use the SDK <code>KVStore</code> and codec through the returned context.</li>
        <li>Retain the returned cache when the caller needs to commit the buffered writes.</li>
        <li>The SDK multistore supplies the cache used for speculative execution over <Link href="/storage">PaxDB storage</Link>.</li>
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
