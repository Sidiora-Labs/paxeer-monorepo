'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useEffect, useRef, useState } from 'react'
import { Search } from './Search'
import { Icon } from './Icon'
import { PaxeerMark } from './PaxeerMark'
import { useLightDismiss } from './useLightDismiss'

function storedTheme() {
  try {
    const value = localStorage.getItem('paxeer-theme')
    return value === 'light' || value === 'dark' ? value : null
  } catch { return null }
}

export function SiteHeader() {
  const pathname = usePathname().replace(/\/$/, '') || '/'
  const [light, setLight] = useState(false)
  const menu = useRef<HTMLDialogElement>(null)
  useLightDismiss(menu)

  function applyTheme(override: 'light' | 'dark' | null) {
    const theme = override ?? (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
    document.querySelector('meta[name="color-scheme"]')?.setAttribute('content', override ?? 'light dark')
    document.documentElement.classList.toggle('light', theme === 'light')
    document.documentElement.classList.toggle('dark', theme === 'dark')
    document.body.dataset.theme = theme
    setLight(theme === 'light')
  }

  useEffect(() => {
    const preference = matchMedia('(prefers-color-scheme: dark)')
    const sync = () => applyTheme(storedTheme())
    const storage = (event: StorageEvent) => { if (event.key === 'paxeer-theme' || event.key === null) sync() }
    sync()
    preference.addEventListener('change', sync)
    window.addEventListener('storage', storage)
    return () => {
      preference.removeEventListener('change', sync)
      window.removeEventListener('storage', storage)
    }
  }, [])

  function toggleTheme() {
    const theme = document.body.dataset.theme === 'light' ? 'dark' : 'light'
    const system = matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
    const override = theme === system ? null : theme
    applyTheme(override)
    try {
      if (override) localStorage.setItem('paxeer-theme', override)
      else localStorage.removeItem('paxeer-theme')
    } catch { /* The current scheme also works without persistence. */ }
  }

  const links = <>
    <Link href="/" aria-current={pathname === '/' ? 'page' : undefined}>Overview</Link>
    <Link href="/sdk-quickstarts" aria-current={pathname === '/sdk-quickstarts' ? 'page' : undefined}>Build</Link>
    <Link href="/platform-api" aria-current={pathname === '/platform-api' ? 'page' : undefined}>API reference</Link>
    <Link href="/operators" aria-current={pathname === '/operators' ? 'page' : undefined}>Operate</Link>
  </>
  return <header className="site-header">
    <div className="header-inner">
      <Link href="/" className="brand" aria-label="Paxeer X documentation home"><PaxeerMark /><span>Paxeer X <span className="brand-suffix">Docs</span></span></Link>
      <nav className="header-nav" aria-label="Main navigation">{links}</nav>
      <div className="header-tools">
        <Search />
        <button type="button" className="icon-button" onClick={toggleTheme} aria-label={light ? 'Switch to dark theme' : 'Switch to light theme'}><Icon name={light ? 'moon' : 'sun'} /></button>
        <button type="button" className="icon-button header-menu-button" aria-label="Open main navigation" aria-haspopup="dialog" aria-controls="main-navigation" onClick={() => menu.current?.showModal()}><Icon name="menu" /></button>
        <a className="header-github" href="https://github.com/Sidiora-Labs/paxeer-monorepo" target="_blank" rel="noreferrer">GitHub <Icon name="arrow" size="sm" /></a>
      </div>
    </div>
    <dialog ref={menu} id="main-navigation" className="header-menu-dialog" aria-labelledby="main-navigation-title">
      <div className="mobile-navigation-top"><h2 id="main-navigation-title">Main navigation</h2><button type="button" className="icon-button" aria-label="Close main navigation" autoFocus onClick={() => menu.current?.close()}><Icon name="close" /></button></div>
      <nav aria-label="Main navigation" onClick={event => { if ((event.target as Element).closest('a')) menu.current?.close() }}>{links}<a href="https://github.com/Sidiora-Labs/paxeer-monorepo" target="_blank" rel="noreferrer">GitHub <Icon name="arrow" size="sm" /></a></nav>
    </dialog>
  </header>
}
