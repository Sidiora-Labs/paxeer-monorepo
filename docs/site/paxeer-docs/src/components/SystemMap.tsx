'use client'

import Link from 'next/link'
import { useCallback, useEffect, useRef, useState } from 'react'
import systems from '@/lib/systems.json'
import { pageTitleForPath } from './nav'
import styles from './SystemMap.module.css'

type System = (typeof systems)[number]
const groups = new Map<string, System[]>()
const systemsById = new Map(systems.map(system => [system.id, system]))
for (const system of systems) {
  const group = groups.get(system.kind)
  if (group) group.push(system)
  else groups.set(system.kind, [system])
}
const changeLabels = { new: 'New', changed: 'Changed', unchanged: 'Unchanged', retired: 'Retired' }

export function SystemMap() {
  const [query, setQuery] = useState('')
  const [kind, setKind] = useState('')
  const [change, setChange] = useState('')
  const revealFrame = useRef(0)

  function clearFilters() {
    setQuery('')
    setKind('')
    setChange('')
  }

  const revealSystem = useCallback((id: string) => {
    if (!id.startsWith('system-') || !systemsById.has(id.slice(7))) return
    setQuery('')
    setKind('')
    setChange('')
    cancelAnimationFrame(revealFrame.current)
    revealFrame.current = requestAnimationFrame(() => {
      const target = document.getElementById(id)
      if (target instanceof HTMLDetailsElement) {
        target.open = true
        target.querySelector('summary')?.focus({ preventScroll: true })
        target.scrollIntoView({ block: 'start' })
      }
    })
  }, [])

  useEffect(() => {
    function revealHash() {
      revealSystem(window.location.hash.slice(1))
    }
    revealHash()
    window.addEventListener('hashchange', revealHash)
    return () => {
      cancelAnimationFrame(revealFrame.current)
      window.removeEventListener('hashchange', revealHash)
    }
  }, [revealSystem])

  const search = query.trim().toLowerCase()
  const matches = (system: System) => (!kind || system.kind === kind)
    && (!change || system.change === change)
    && (!search || `${system.title} ${system.id}`.toLowerCase().includes(search))
  const count = systems.filter(matches).length

  return <div className={styles.map}>
    <div className={styles.filters} role="search" aria-label="Find a system">
      <label className={styles.search}>Find by name
        <input type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder="System name or identifier" />
      </label>
      <label>Kind
        <select value={kind} onChange={event => setKind(event.target.value)}>
          <option value="">All kinds</option>
          {Array.from(groups.keys()).map(value => <option key={value} value={value}>{value}</option>)}
        </select>
      </label>
      <label>Change in 7.0.0
        <select value={change} onChange={event => setChange(event.target.value)}>
          <option value="">All changes</option>
          {Object.entries(changeLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </select>
      </label>
      <button type="button" className="pill-button small-button" onClick={clearFilters}>Clear filters</button>
    </div>
    <p role="status" aria-live="polite" aria-atomic="true">{count} of {systems.length} systems</p>
    {count === 0 && <p>No systems match these filters. Clear the filters or try another name.</p>}
    {Array.from(groups, ([groupKind, members]) => <section key={groupKind} hidden={!members.some(matches)} aria-labelledby={`kind-${groupKind}`}>
      <h2 id={`kind-${groupKind}`} className={styles.kind}>{groupKind} <span>({members.filter(matches).length})</span></h2>
      {members.map(system => <details key={system.id} id={`system-${system.id}`} hidden={!matches(system)} className={styles.system}>
        <summary><strong>{system.title}</strong> <span className={styles.change} data-change={system.change}>{changeLabels[system.change as keyof typeof changeLabels]}</span></summary>
        <div className={styles.details}>
          <p><code>{system.id}</code></p>
          <dl>
            <dt>Directory</dt><dd><code>{system.path}</code></dd>
            <dt>Build command</dt><dd><pre tabIndex={0} aria-label={`Build command for ${system.title}`}><code>{system.build}</code></pre></dd>
            <dt>Build output</dt><dd><code>{system.artefact}</code></dd>
            <dt>Configuration</dt><dd><code>{system.configuration}</code></dd>
            <dt>Ports</dt><dd>{system.ports.length ? system.ports.map(port => <code key={port} className={styles.port}>{port}</code>) : 'None listed'}</dd>
            <dt>Health check</dt><dd><code>{system.health}</code></dd>
            <dt>Depends on</dt><dd>{system.depends.length ? <ul>{system.depends.map(id => <li key={id}><a href={`#system-${id}`} onClick={() => revealSystem(`system-${id}`)}>{systemsById.get(id)!.title}</a></li>)}</ul> : 'None'}</dd>
            <dt>Site pages</dt><dd>{system.siteDocs.length ? <ul>{system.siteDocs.map(href => <li key={href}><Link href={href}>{pageTitleForPath(href)}</Link></li>)}</ul> : 'See repository references below.'}</dd>
            <dt>Repository references</dt><dd><ul>{system.docs.filter(reference => !reference.startsWith('/')).map(reference => <li key={reference}><code>{reference}</code></li>)}</ul></dd>
            {system.unpublishedDocs.length > 0 && <><dt>Additional inventory references</dt><dd>{system.unpublishedDocs.map(route => <span key={route}><code>{route}</code> (no published site page)</span>)}</dd></>}
          </dl>
        </div>
      </details>)}
    </section>)}
  </div>
}
