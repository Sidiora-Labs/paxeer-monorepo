'use client'

import Link from 'next/link'
import { ReactNode, useEffect, useRef, useState } from 'react'
import { usePathname } from 'next/navigation'
import { Sidebar } from './Sidebar'
import { CopyForAgent } from './CopyForAgent'
import { navStructure, pageTitleForPath } from './nav'
import { Icon } from './Icon'
import { jsonLd } from '@/lib/seo'
import { DocFigure, figureForTopic } from './DocFigure'

type Heading = { id: string; text: string; level: number }
export function DocsLayout({ children }: { children: ReactNode }) {
  const pathname = usePathname().replace(/\/$/, '') || '/'
  const [navigationOpen, setNavigationOpen] = useState(false)
  const [headings, setHeadings] = useState<Heading[]>([])
  const [active, setActive] = useState('')
  const article = useRef<HTMLElement>(null)
  const pageTitle = pageTitleForPath(pathname)
  const topicFigure = figureForTopic(pathname)
  const related = navStructure.find(group => group.items.some(item => item.href === pathname))?.items.filter(item => item.href !== pathname).slice(0, 2) || []
  const section = navStructure.find(group => group.items.some(item => item.href === pathname))?.section

  useEffect(() => {
    const counts = new Map<string, number>()
    const nodes = Array.from(article.current?.querySelectorAll<HTMLElement>('h2,h3,h4') || [])
    const links: HTMLElement[] = []
    const entries = nodes.map(node => {
      const text = node.textContent?.trim() || ''
      const slug = text.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'section'
      const count = (counts.get(slug) || 0) + 1
      counts.set(slug, count)
      node.id = node.id || (count === 1 ? slug : `${slug}-${count}`)
      const anchor = document.createElement('a')
      anchor.href = `#${node.id}`
      anchor.className = 'heading-anchor'
      anchor.setAttribute('aria-label', `Link to ${text}`)
      anchor.textContent = '#'
      node.append(anchor)
      links.push(anchor)
      return { id: node.id, text, level: Number(node.tagName.slice(1)) }
    })
    setHeadings(entries)
    setActive(entries[0]?.id || '')
    const observer = new IntersectionObserver(changes => {
      const visible = changes.filter(entry => entry.isIntersecting)
      if (visible[0]) setActive(visible[0].target.id)
    }, { rootMargin: '-100px 0px -60% 0px' })
    nodes.forEach(node => observer.observe(node))
    const frame = requestAnimationFrame(() => {
      if (window.location.hash) document.getElementById(decodeURIComponent(window.location.hash.slice(1)))?.scrollIntoView()
    })
    return () => { cancelAnimationFrame(frame); observer.disconnect(); links.forEach(link => link.remove()) }
  }, [pathname, children])

  const contents = <nav aria-label="On this page">{headings.map(heading => <a href={`#${heading.id}`} key={heading.id} className={heading.level > 2 ? 'toc-child' : undefined} aria-current={active === heading.id ? 'location' : undefined} onClick={() => setActive(heading.id)}>{heading.text}</a>)}</nav>
  return <div className="docs-shell">
    <Sidebar open={navigationOpen} onClose={() => setNavigationOpen(false)} />
    <main id="main-content" className="docs-main">
      <div className="docs-toolbar"><div className="breadcrumbs"><button type="button" className="docs-menu-button" aria-controls="mobile-docs-navigation" aria-expanded={navigationOpen} onClick={() => setNavigationOpen(true)}><Icon name="menu" />Menu</button><Link href="/">Docs</Link><span>/</span><span>{section}</span></div><CopyForAgent pageTitle={pageTitle}>{children}</CopyForAgent></div>
      <div className="docs-reading-grid">
        <div className="article-column">
          <details className="mobile-toc"><summary>On this page</summary>{contents}</details>
          <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: jsonLd(pathname) }} /><DocFigure key={pathname} {...topicFigure} /><article ref={article} className="article">{children}</article>
          {related.length > 0 && <section className="doc-related" aria-label="Continue exploring"><p>Continue exploring</p><DocFigure kind="branches" label="Connected paths representing related documentation" caption="Follow the connections across Paxeer X." compact /><div className="doc-related-links">{related.map(item => <Link key={item.href} href={item.href}>{item.title} <span aria-hidden="true">↗</span></Link>)}</div></section>}
          <div className="docs-bottom"><span>Paxeer X · System documentation</span><a href="#main-content">Back to top ↑</a></div>
        </div>
        <aside className="table-of-contents"><p>ON THIS PAGE</p>{contents}<div className="toc-resources"><Icon name="code" /><span>Working with an agent?</span><small>Copy this page into your coding assistant for context.</small></div></aside>
      </div>
    </main>
  </div>
}
