'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useEffect, useRef } from 'react'
import { navStructure } from './nav'
import { Icon } from './Icon'

export function Sidebar({ open, onClose }: { open: boolean; onClose: () => void }) {
  const pathname = usePathname().replace(/\/$/, '') || '/'
  const dialog = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    if (open) dialog.current?.showModal()
    else dialog.current?.close()
  }, [open])
  const navigation = <>
    <div className="sidebar-heading"><span className="sidebar-label">PAXEER X DOCUMENTATION</span><span className="chain-badge">Chain 125</span></div>
    <nav aria-label="Documentation">{navStructure.map(section => <div className="sidebar-group" key={section.section}><h2>{section.section}</h2>{section.items.map(item => <Link key={item.href} href={item.href} onClick={onClose} aria-current={pathname === item.href ? 'page' : undefined}>{item.title}</Link>)}</div>)}</nav>
    <Link className="sidebar-source" href="/network-parameters"><Icon name="layers" /><span>Network reference<small>Identifiers, assets & configuration</small></span></Link>
  </>
  return <>
    <aside className="docs-sidebar">{navigation}</aside>
    <dialog ref={dialog} id="mobile-docs-navigation" aria-label="Documentation navigation" className="mobile-navigation" onCancel={onClose} onClose={onClose} onClick={event => { if (event.target === event.currentTarget) onClose() }}><div className="mobile-navigation-content"><div className="mobile-navigation-top"><span>Paxeer X Docs</span><button className="icon-button" onClick={onClose} aria-label="Close navigation"><Icon name="close" /></button></div>{navigation}</div></dialog>
  </>
}
