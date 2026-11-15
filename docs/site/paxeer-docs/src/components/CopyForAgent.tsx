'use client'

import { CopyControl } from '@centra/docs-assistant/react'
import { copyText } from '@/lib/clipboard'

const blockTags = new Set(['ADDRESS', 'ARTICLE', 'BLOCKQUOTE', 'DD', 'DETAILS', 'DIV', 'DL', 'DT', 'FIGCAPTION', 'FIGURE', 'FOOTER', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'HEADER', 'HR', 'LI', 'MAIN', 'OL', 'P', 'PRE', 'SECTION', 'SUMMARY', 'TABLE', 'UL'])

function inlineMarkdown(node: Node): string {
  if (node.nodeType === Node.TEXT_NODE) return (node.textContent ?? '').replace(/\s+/g, ' ')
  if (!(node instanceof Element)) return ''
  const text = Array.from(node.childNodes).map(inlineMarkdown).join('')
  if (node.tagName === 'BR') return '  \n'
  if (node.tagName === 'CODE') {
    const code = node.textContent ?? ''
    const longest = Math.max(0, ...(code.match(/`+/g) ?? []).map((match) => match.length))
    const fence = '`'.repeat(longest + 1)
    const padding = code.startsWith('`') || code.endsWith('`') ? ' ' : ''
    return `${fence}${padding}${code}${padding}${fence}`
  }
  if (node.tagName === 'STRONG' || node.tagName === 'B') return `**${text.trim()}**`
  if (node.tagName === 'EM' || node.tagName === 'I') return `*${text.trim()}*`
  if (node.tagName === 'DEL' || node.tagName === 'S') return `~~${text.trim()}~~`
  if (node.tagName === 'A') {
    const href = node.getAttribute('href')
    return href ? `[${text.trim()}](<${new URL(href, window.location.href).href}>)` : text
  }
  if (node.tagName === 'IMG') {
    const src = node.getAttribute('src')
    return src ? `![${node.getAttribute('alt') ?? ''}](<${new URL(src, window.location.href).href}>)` : ''
  }
  return text
}

function markdownBlocks(element: Element): string {
  const blocks: string[] = []
  let inline = ''
  const flush = () => {
    if (inline.trim()) blocks.push(inline.trim())
    inline = ''
  }
  for (const node of Array.from(element.childNodes)) {
    if (!(node instanceof Element) || !blockTags.has(node.tagName)) {
      inline += inlineMarkdown(node)
      continue
    }
    flush()
    let block: string
    if (/^H[1-6]$/.test(node.tagName)) {
      block = `${'#'.repeat(Number(node.tagName.slice(1)))} ${inlineMarkdown(node).trim()}`
    } else if (node.tagName === 'PRE') {
      const code = node.querySelector('code')
      const text = (code ?? node).textContent ?? ''
      const language = Array.from(code?.classList ?? []).find((name) => name.startsWith('language-'))?.slice(9) ?? ''
      const longest = Math.max(0, ...(text.match(/`+/g) ?? []).map((match) => match.length))
      const fence = '`'.repeat(Math.max(3, longest + 1))
      block = `${fence}${language}\n${text}${text.endsWith('\n') ? '' : '\n'}${fence}`
    } else if (node.tagName === 'UL' || node.tagName === 'OL') {
      const start = Number.parseInt(node.getAttribute('start') ?? '1', 10) || 1
      block = Array.from(node.children).filter((child) => child.tagName === 'LI').map((item, index) => {
        const prefix = node.tagName === 'OL' ? `${start + index}. ` : '- '
        const content = markdownBlocks(item)
        return prefix + content.replace(/\n/g, `\n${' '.repeat(prefix.length)}`)
      }).join('\n')
    } else if (node.tagName === 'TABLE') {
      const rows = Array.from(node.querySelectorAll('tr')).map((row) => Array.from(row.children).filter((cell) => cell.tagName === 'TH' || cell.tagName === 'TD').map((cell) => inlineMarkdown(cell).trim().replace(/\n+/g, '<br>').replace(/\|/g, '\\|'))).filter((row) => row.length)
      const width = Math.max(0, ...rows.map((row) => row.length))
      const row = (cells: string[]) => `| ${Array.from({ length: width }, (_, index) => cells[index] ?? '').join(' | ')} |`
      block = rows.length ? [row(rows[0]), row(Array(width).fill('---')), ...rows.slice(1).map(row)].join('\n') : ''
    } else if (node.tagName === 'BLOCKQUOTE') {
      block = markdownBlocks(node).split('\n').map((line) => `> ${line}`).join('\n')
    } else if (node.tagName === 'HR') {
      block = '---'
    } else {
      block = markdownBlocks(node)
    }
    if (block) blocks.push(block)
  }
  flush()
  return blocks.join('\n\n')
}

export function CopyForAgent({ pageTitle }: { pageTitle: string; children: React.ReactNode }) {
  const handleCopy = async (article: HTMLElement) => {
      const clone = article.cloneNode(true) as HTMLElement
      clone.querySelectorAll('nav, aside, button, script, style, svg, .copy-for-agent-btn, .copy-button, .docs-copy-control, .heading-anchor, .prev-next').forEach((element) => element.remove())
      const heading = clone.querySelector('h1')
      const title = heading ? inlineMarkdown(heading).trim() : pageTitle
      heading?.remove()
      const markdown = `# ${title}\n\nSource: <${window.location.href}>\n\n${markdownBlocks(clone)}\n`
      await copyText(markdown)
  }

  return <CopyControl targetId="documentation-article" resetKey={pageTitle} label="Copy for agent" className="copy-for-agent-btn" title="Copy this page as Markdown for your coding assistant" onCopy={handleCopy} />
}
