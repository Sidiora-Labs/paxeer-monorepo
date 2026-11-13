'use client'

import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { useRouter } from 'next/navigation'

interface SearchEntry {
  title: string
  pageTitle: string
  href: string
  section: string
  content: string
  heading: boolean
}

function excerpt(content: string, terms: string[]) {
  const lower = content.toLowerCase()
  const matches = terms.map((term) => lower.indexOf(term)).filter((index) => index >= 0)
  const start = matches.length ? Math.max(0, Math.min(...matches) - 48) : 0
  const text = content.slice(start, start + 180)
  return `${start ? '…' : ''}${text}${start + 180 < content.length ? '…' : ''}`
}

export function Search() {
  const [isOpen, setIsOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [section, setSection] = useState('All docs')
  const [entries, setEntries] = useState<SearchEntry[] | null>(null)
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')
  const [retry, setRetry] = useState(0)
  const [selectedIndex, setSelectedIndex] = useState(0)
  const router = useRouter()
  const dialogRef = useRef<HTMLDialogElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const resultRefs = useRef<(HTMLAnchorElement | null)[]>([])
  const dialogTitleId = useId()
  const resultsId = useId()
  const helpId = useId()
  const terms = useMemo(() => query.toLowerCase().trim().split(/\s+/).filter(Boolean), [query])
  const sections = useMemo(() => ['All docs', ...new Set((entries ?? []).map((entry) => entry.section))], [entries])
  const matches = useMemo(() => {
    const candidates = (entries ?? []).filter((entry) => section === 'All docs' || entry.section === section)
    if (!terms.length) return candidates.filter((entry) => !entry.heading)
    return candidates.flatMap((entry) => {
      const title = entry.title.toLowerCase()
      const pageTitle = entry.pageTitle.toLowerCase()
      const content = entry.content.toLowerCase()
      const searchable = `${title} ${pageTitle} ${entry.section.toLowerCase()} ${content}`
      if (!terms.every((term) => searchable.includes(term))) return []
      const score = terms.reduce((total, term) => total + (title.includes(term) ? 8 : 0) + (pageTitle.includes(term) ? 3 : 0) + (content.includes(term) ? 1 : 0), 0)
        + (title === query.trim().toLowerCase() ? 20 : 0)
      return [{ ...entry, score }]
    }).sort((a, b) => b.score - a.score)
  }, [entries, query, section, terms])
  const results = matches

  useEffect(() => {
    const handleShortcut = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === 'k' && (event.metaKey || event.ctrlKey)) {
        event.preventDefault()
        setIsOpen((open) => !open)
      }
    }
    document.addEventListener('keydown', handleShortcut)
    return () => document.removeEventListener('keydown', handleShortcut)
  }, [])

  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return
    if (!isOpen) {
      if (dialog.open) dialog.close()
      return
    }
    if (!dialog.open) dialog.showModal()
    inputRef.current?.focus()
    const overflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = overflow }
  }, [isOpen])

  useEffect(() => {
    if (!isOpen || entries) return
    const controller = new AbortController()
    setStatus('loading')
    fetch('/search-index.json', { signal: controller.signal })
      .then(async (response) => {
        if (!response.ok) throw new Error('Search index unavailable')
        const data: unknown = await response.json()
        if (!Array.isArray(data) || !data.every((entry) => entry !== null && typeof entry === 'object' && typeof entry.title === 'string' && typeof entry.pageTitle === 'string' && typeof entry.href === 'string' && typeof entry.section === 'string' && typeof entry.content === 'string' && typeof entry.heading === 'boolean')) {
          throw new Error('Search index invalid')
        }
        setEntries(data)
        setStatus('ready')
      })
      .catch(() => { if (!controller.signal.aborted) setStatus('error') })
    return () => controller.abort()
  }, [isOpen, entries, retry])

  useEffect(() => {
    resultRefs.current[selectedIndex]?.scrollIntoView({ block: 'nearest' })
  }, [selectedIndex])

  const select = (href: string) => {
    setIsOpen(false)
    setQuery('')
    setSection('All docs')
    setSelectedIndex(0)
    router.push(href)
  }

  return (
    <>
      <button type="button" className="search-trigger" aria-haspopup="dialog" aria-expanded={isOpen} aria-label="Search documentation" onClick={() => setIsOpen(true)}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <circle cx="10.5" cy="10.5" r="6.5" stroke="currentColor" strokeWidth="1.7" />
          <path d="m16 16 4.5 4.5" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
        </svg>
        <span>Search docs</span><kbd>⌘ K</kbd>
      </button>
      <dialog
        ref={dialogRef}
        className="search-dialog"
        aria-labelledby={dialogTitleId}
        onCancel={() => setIsOpen(false)}
        onClose={() => setIsOpen(false)}
        onClick={(event) => {
          if (event.target !== event.currentTarget) return
          const rect = event.currentTarget.getBoundingClientRect()
          if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) setIsOpen(false)
        }}
      >
        <h2 id={dialogTitleId} className="sr-only">Search Paxeer documentation</h2>
        <div className="search-input-row">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <circle cx="10.5" cy="10.5" r="6.5" stroke="currentColor" strokeWidth="1.7" />
            <path d="m16 16 4.5 4.5" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
          </svg>
          <input
            ref={inputRef}
            value={query}
            onChange={(event) => { setQuery(event.target.value); setSelectedIndex(0) }}
            onKeyDown={(event) => {
              if (!results.length) return
              if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
                event.preventDefault()
                setSelectedIndex((index) => (index + (event.key === 'ArrowDown' ? 1 : -1) + results.length) % results.length)
              } else if (event.key === 'Enter' && results[selectedIndex]) {
                event.preventDefault()
                select(results[selectedIndex].href)
              }
            }}
            type="search"
            role="combobox"
            aria-label="Search documentation"
            aria-autocomplete="list"
            aria-controls={resultsId}
            aria-describedby={helpId}
            aria-expanded={results.length > 0}
            aria-activedescendant={results[selectedIndex] ? `${resultsId}-${selectedIndex}` : undefined}
            autoComplete="off"
            spellCheck={false}
            placeholder="Search guides, methods, or commands…"
          />
          <button type="button" aria-label="Close search" onClick={() => setIsOpen(false)}><kbd>Esc</kbd></button>
        </div>
        <div className="search-filters" aria-label="Filter search by documentation section">
          {sections.map((name) => (
            <button key={name} type="button" aria-pressed={section === name} onClick={() => { setSection(name); setSelectedIndex(0); inputRef.current?.focus() }}>{name}</button>
          ))}
        </div>
        {status === 'loading' && <p className="search-empty" role="status">Loading documentation…</p>}
        {status === 'error' && <div className="search-empty" role="status"><p>Search could not load. Please try again.</p><button type="button" onClick={() => setRetry((value) => value + 1)}>Retry search</button></div>}
        {status === 'ready' && results.length === 0 && <div className="search-empty" role="status"><strong>No results for “{query.trim()}”</strong><p>Try a method name, command, or a different section.</p></div>}
        <div id={resultsId} role="listbox" aria-label="Documentation results" className="search-results">
          {status === 'ready' && results.map((result, index) => (
            <a
              id={`${resultsId}-${index}`}
              ref={(node) => { resultRefs.current[index] = node }}
              key={result.href}
              href={result.href}
              role="option"
              aria-selected={index === selectedIndex}
              tabIndex={-1}
              className="search-result"
              onMouseMove={() => setSelectedIndex(index)}
              onClick={(event) => {
                if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
                event.preventDefault()
                select(result.href)
              }}
            >
              <span className="search-result-section">{result.section}{result.heading ? ` / ${result.pageTitle}` : ''}</span>
              <strong>{result.title}</strong>
              <span className="search-result-excerpt">{excerpt(result.content, terms)}</span>
            </a>
          ))}
        </div>
        <div className="search-footer">
          <span id={helpId}><kbd>↑</kbd><kbd>↓</kbd> Navigate <kbd>↵</kbd> Open</span>
          <span role="status">{status === 'ready' ? (terms.length ? `${matches.length} result${matches.length === 1 ? '' : 's'}` : 'Explore the documentation') : 'Paxeer docs'}</span>
        </div>
      </dialog>
    </>
  )
}
