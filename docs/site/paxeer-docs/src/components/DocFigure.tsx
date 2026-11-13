'use client'

import { useEffect, useRef, useState } from 'react'
import type { Figure } from '@/lib/hairline'
import '@/styles/doc-figures.css'

export type DocFigureKind = 'terrain' | 'exploded' | 'branches' | 'vault' | 'router' | 'plot' | 'terminal' | 'cabinet' | 'riffle' | 'padlock' | 'drawer' | 'plug' | 'basket' | 'loupe' | 'dish'

/** The supplied Hairline engines, mounted only near the viewport. */
export function DocFigure({ kind = 'exploded', label, caption, compact = false }: { kind?: DocFigureKind; label: string; caption?: string; compact?: boolean }) {
  const host = useRef<HTMLDivElement>(null)
  const [paused, setPaused] = useState(false)
  useEffect(() => {
    const element = host.current
    if (!element) return
    let figure: Figure | undefined
    let disposed = false
    let visible = false
    const mount = async () => {
      if (disposed || !visible || paused || figure) return
      const library = await import('@/lib/hairline')
      if (disposed || !visible || paused || figure) return
      element.replaceChildren()
      figure = library[kind](element, { label, intensity: 0.35, theme: 'auto' })
    }
    const observer = new IntersectionObserver(entries => {
      visible = entries[0]?.isIntersecting || false
      if (visible) void mount()
      // Engines also share an offscreen-aware frame loop; retain their pose.
    }, { rootMargin: '120px' })
    observer.observe(element)
    return () => {
      disposed = true
      observer.disconnect()
      const snapshot = element.querySelector('svg')?.cloneNode(true)
      figure?.destroy()
      element.replaceChildren()
      if (snapshot) element.appendChild(snapshot)
    }
  }, [kind, label, paused])
  return <figure className={`doc-figure${compact ? ' doc-figure--compact' : ''}`}>
    <div className="doc-figure-stage" ref={host} data-hairline={kind} role={kind === 'riffle' && !paused ? 'group' : 'img'} aria-label={label} />
    <figcaption><span>{caption || label}</span><button type="button" onClick={() => setPaused(value => !value)} aria-pressed={paused} aria-label={`${paused ? 'Resume' : 'Pause'} illustration motion`}>{paused ? 'Resume motion' : 'Pause motion'}</button></figcaption>
  </figure>
}

export function figureForTopic(path: string): { kind: DocFigureKind; label: string; caption: string } {
  if (/security|permission|identity|compliance|governance/.test(path)) return { kind: 'padlock', label: 'A lock representing policy and permissions', caption: 'Explicit authority. Verifiable policy.' }
  if (/wallet|custody|account|vault/.test(path)) return { kind: 'vault', label: 'A vault representing protected accounts and assets', caption: 'Control assets and account authority.' }
  if (/agent|business|autonom/.test(path)) return { kind: 'branches', label: 'A branching graph representing autonomous business coordination', caption: 'Coordinate agents, businesses, and decisions.' }
  if (/market|exchange|trading|liquidity|order|instrument|risk/.test(path)) return { kind: 'plot', label: 'A market chart representing instruments and exchange activity', caption: 'Discover, price, and exchange value.' }
  if (/bridge|cross|routing|settlement|network|interop/.test(path)) return { kind: 'router', label: 'A network router representing connected execution and settlement', caption: 'Connect execution, messages, and settlement.' }
  if (/storage|data|index|archive/.test(path)) return { kind: 'drawer', label: 'A filing system representing durable system records', caption: 'Persist records. Retrieve system state.' }
  if (/node|operator|configuration|installation|validator/.test(path)) return { kind: 'cabinet', label: 'A server cabinet representing Paxeer X infrastructure', caption: 'Operate the infrastructure behind the system.' }
  if (/api|sdk|python|hardhat|foundry|debug|frontend|quickstart|cli/.test(path)) return { kind: 'terminal', label: 'A terminal representing developer tools and integrations', caption: 'Build with familiar tools and explicit interfaces.' }
  if (/oracle|query|observ|monitor/.test(path)) return { kind: 'loupe', label: 'A magnifying lens representing system inspection', caption: 'Inspect signals, evidence, and system state.' }
  if (/consensus|parallel|engine|execution|evm|wasm/.test(path)) return { kind: 'terrain', label: 'A field of pillars representing coordinated execution', caption: 'Coordinate computation across the execution layer.' }
  if (/token|mint|asset|payment/.test(path)) return { kind: 'riffle', label: 'A tray of cards representing issued assets; use arrow keys to explore', caption: 'Issue assets and compose their utility.' }
  return { kind: 'exploded', label: 'Layered architecture representing the integrated Paxeer X system', caption: 'One system. Composable layers.' }
}
