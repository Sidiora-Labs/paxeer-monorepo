'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useEffect, useState } from 'react'
import { Search } from './Search'
import { Icon } from './Icon'
import { PaxeerMark } from './PaxeerMark'

export function SiteHeader() {
  const pathname = usePathname().replace(/\/$/, '') || '/'
  const [light, setLight] = useState(false)
  useEffect(() => { setLight(document.body.dataset.theme === 'light') }, [])
  function toggleTheme() {
    const next = !light
    const theme = next ? 'light' : 'dark'
    document.documentElement.classList.remove('light', 'dark')
    document.documentElement.classList.add(theme)
    document.documentElement.dataset.pendingTheme = theme
    document.body.dataset.theme = theme
    try { localStorage.setItem('paxeer-theme', next ? 'light' : 'dark') } catch {}
    setLight(next)
  }
  return <header className="site-header">
    <div className="header-inner">
      <Link href="/" className="brand" aria-label="Paxeer X documentation home"><PaxeerMark /><span>Paxeer X <span className="brand-suffix">Docs</span></span></Link>
      <nav className="header-nav" aria-label="Main navigation">
        <Link href="/" aria-current={pathname === '/' ? 'page' : undefined}>Overview</Link>
        <Link href="/sdk-quickstarts" aria-current={pathname === '/sdk-quickstarts' ? 'page' : undefined}>Build</Link>
        <Link href="/platform-api" aria-current={pathname === '/platform-api' ? 'page' : undefined}>API reference</Link>
        <Link href="/operators" aria-current={pathname === '/operators' ? 'page' : undefined}>Operate</Link>
      </nav>
      <div className="header-tools"><Search /><button className="icon-button" onClick={toggleTheme} aria-label={light ? 'Switch to dark theme' : 'Switch to light theme'}><Icon name={light ? 'moon' : 'sun'} /></button><a className="header-github" href="https://github.com/Sidiora-Labs/paxeer-monorepo" target="_blank" rel="noreferrer">GitHub <Icon name="arrow" size={14} /></a></div>
    </div>
  </header>
}
