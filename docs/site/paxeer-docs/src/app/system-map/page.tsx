import { DocsLayout } from '@/components/DocsLayout'
import { SystemMap } from '@/components/SystemMap'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/system-map')

export default function SystemMapPage() {
  return <DocsLayout>
    <div className="page-header">
      <h1 className="page-title">System map</h1>
      <p className="page-description">Explore every Paxeer 7.0.0 system by kind, release change, and name, with build commands, configuration, health checks, dependencies, and documentation.</p>
    </div>
    <p>Choose a system to expand its details. Follow a dependency to open that system, including when it is outside the current filters. Build commands run from the repository root unless they include a change of directory.</p>
    <SystemMap />
  </DocsLayout>
}
