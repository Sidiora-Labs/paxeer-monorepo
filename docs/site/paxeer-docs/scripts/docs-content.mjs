import { readdir, readFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

export const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const entities = { amp: '&', lt: '<', gt: '>', quot: '"', apos: "'", nbsp: ' ' }
const decode = text => text.replace(/&(#x[\da-f]+|#\d+|amp|lt|gt|quot|apos|nbsp);/gi, (raw, name) => {
  if (!name.startsWith('#')) return entities[name.toLowerCase()] || raw
  const value = name[1].toLowerCase() === 'x' ? parseInt(name.slice(2), 16) : parseInt(name.slice(1), 10)
  return value > 0 && value <= 0x10ffff ? String.fromCodePoint(value) : raw
})
export const clean = text => decode(text).replace(/\s+/g, ' ').trim()
const tag = node => ts.isJsxElement(node) ? node.openingElement.tagName.getText() : ts.isJsxSelfClosingElement(node) ? node.tagName.getText() : ''
const attr = (node, name) => {
  const attributes = ts.isJsxElement(node) ? node.openingElement.attributes : ts.isJsxSelfClosingElement(node) ? node.attributes : null
  const found = attributes?.properties.find(item => ts.isJsxAttribute(item) && item.name.getText() === name)
  return found?.initializer ? plain(found.initializer) : ''
}
function plain(node) {
  if (ts.isJsxText(node) || ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) return node.text
  if (ts.isJsxExpression(node)) return node.expression ? plain(node.expression) : ''
  if (ts.isJsxElement(node) || ts.isJsxFragment(node)) return node.children.map(plain).join('')
  if (ts.isJsxSelfClosingElement(node) && tag(node) === 'br') return ' '
  return ''
}
function markdown(node) {
  if (ts.isJsxText(node)) return decode(node.text).replace(/\s+/g, ' ')
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) return node.text
  if (ts.isJsxExpression(node)) return node.expression ? markdown(node.expression) : ''
  if (ts.isJsxFragment(node)) return node.children.map(markdown).join('')
  if (ts.isJsxSelfClosingElement(node)) return tag(node) === 'br' ? '\n' : ''
  if (!ts.isJsxElement(node)) return ''
  const name = tag(node)
  if (['script', 'style', 'svg', 'nav', 'footer'].includes(name) || /prev-next|copy-/.test(attr(node, 'className'))) return ''
  const content = node.children.map(markdown).join('')
  if (/^h[1-6]$/.test(name)) return `\n\n${'#'.repeat(Number(name[1]))} ${clean(plain(node))}\n\n`
  if (name === 'pre') {
    const text = decode(plain(node)).trim()
    const longest = Math.max(0, ...(text.match(/`+/g) || []).map(value => value.length))
    const fence = '`'.repeat(Math.max(3, longest + 1))
    return `\n\n${fence}\n${text}\n${fence}\n\n`
  }
  if (name === 'code') return '`' + clean(plain(node)) + '`'
  if (name === 'strong' || name === 'b') return `**${content.trim()}**`
  if (name === 'em') return `*${content.trim()}*`
  if (name === 'Link' || name === 'a') { const href = attr(node, 'href'); return href ? `[${content.trim()}](${href})` : content }
  if (name === 'ol') {
    let number = Number.parseInt(attr(node, 'start') || '1', 10)
    return '\n\n' + node.children.filter(item => tag(item) === 'li').map(item => `${number++}. ${item.children.map(markdown).join('').trim()}`).join('\n') + '\n\n'
  }
  if (name === 'li') return `\n- ${content.trim()}\n`
  if (name === 'table') {
    const rows = []
    function collectRows(child) {
      if (tag(child) === 'tr') {
        rows.push(child.children.filter(item => ts.isJsxElement(item) && ['th','td'].includes(tag(item))).map(item => markdown(item).trim().replace(/\n/g, ' ').replace(/\|/g, '\\|')))
      } else ts.forEachChild(child, collectRows)
    }
    collectRows(node)
    if (!rows.length) return ''
    const row = values => `| ${values.join(' | ')} |`
    return `\n\n${[row(rows[0]),row(rows[0].map(() => '---')),...rows.slice(1).map(row)].join('\n')}\n\n`
  }
  if (['p','div','section','ul','ol','blockquote','DocsLayout'].includes(name)) return `\n\n${content.trim()}\n\n`
  return content
}
async function pageFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true })
  return (await Promise.all(entries.map(entry => entry.isDirectory() ? pageFiles(path.join(directory, entry.name)) : entry.name === 'page.tsx' ? [path.join(directory, entry.name)] : []))).flat().sort()
}
export async function loadPages() {
  const pages = []
  for (const file of await pageFiles(path.join(root, 'src/app'))) {
    const relative = path.relative(path.join(root,'src/app'),path.dirname(file)).split(path.sep).filter(part => !part.startsWith('(')).join('/')
    const route = relative ? '/' + relative : '/'
    const source = ts.createSourceFile(file, await readFile(file,'utf8'),ts.ScriptTarget.Latest,true,ts.ScriptKind.TSX)
    let title = '', description = '', body = '', intro = '', contentRoot
    const headings = [], counts = new Map()
    function visit(node) {
      if (tag(node) === 'h1') title = clean(plain(node))
      if (tag(node) === 'p' && !intro) intro = clean(plain(node))
      if (tag(node) === 'p' && attr(node,'className').includes('page-description')) description = clean(plain(node))
      if (tag(node) === 'DocsLayout' || (!contentRoot && tag(node) === 'main')) contentRoot = node
      ts.forEachChild(node, visit)
    }
    visit(source)
    description ||= intro
    if (!title || !description || !contentRoot) throw new Error(`Add a title, page-description and main or DocsLayout content for ${route}`)
    function collectHeadings(node) {
      if (/^h[2-4]$/.test(tag(node))) {
        const title = clean(plain(node))
        const base = title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'section'
        const count = (counts.get(base) || 0) + 1
        counts.set(base, count)
        // DocsLayout assigns IDs to content headings; homepage headings need explicit IDs.
        const id = attr(node, 'id') || (route !== '/' ? (count === 1 ? base : `${base}-${count}`) : '')
        headings.push({ title, id })
      }
      ts.forEachChild(node, collectHeadings)
    }
    collectHeadings(contentRoot)
    body = markdown(contentRoot)
    const systems = route === '/system-map' ? JSON.parse(await readFile(path.join(root, 'src/lib/systems.json'), 'utf8')) : []
    if (systems.length) {
      const kinds = [...new Set(systems.map(system => system.kind))]
      for (const kind of kinds) {
        headings.push({ title: kind, id: `kind-${kind}` })
        body += `\n\n## ${kind}\n\n`
        for (const system of systems.filter(system => system.kind === kind)) {
          headings.push({ title: system.title, id: `system-${system.id}` })
          body += [
            `### ${system.title}`, '',
            `Identifier: ${system.id}`, `Change in 7.0.0: ${system.change}`, '',
            `Directory: ${system.path}`, '', 'Build command:', '', '```sh', system.build, '```', '',
            `Build output: ${system.artefact}`, '', `Configuration: ${system.configuration}`, '',
            `Ports: ${system.ports.join(', ') || 'None listed'}`, '', `Health check: ${system.health}`, '',
            `Depends on: ${system.depends.map(id => `[${systems.find(item => item.id === id).title}](/system-map#system-${id})`).join(', ') || 'None'}`, '',
            `Site pages: ${system.siteDocs.map(href => `[${href}](${href})`).join(', ') || 'See repository references.'}`, '',
            `Repository references: ${system.docs.filter(reference => !reference.startsWith('/')).join(', ')}`, '',
            ...system.unpublishedDocs.map(route => `Additional inventory reference: ${route} (no published site page)`), '',
          ].join('\n')
        }
      }
    }
    pages.push({route,file,title,description,headings,systems,body:body.replace(/[ \t]+$/gm,'').replace(/\n[ \t]+\n/g,'\n\n').replace(/\n{3,}/g,'\n\n').trim()+'\n'})
  }
  return pages
}
