import { readFile, mkdir, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'
import { loadPages } from './docs-content.mjs'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const appRoot = path.join(root, 'src/app')
const entities = { amp: '&', lt: '<', gt: '>', quot: '"', apos: "'", nbsp: ' ' }
const clean = (text) => text.replace(/&(#x[\da-f]+|#\d+|amp|lt|gt|quot|apos|nbsp);/gi, (match, entity) => {
  if (entity[0] !== '#') return entities[entity.toLowerCase()] ?? match
  const value = entity[1].toLowerCase() === 'x' ? parseInt(entity.slice(2), 16) : parseInt(entity.slice(1), 10)
  return value <= 0x10ffff ? String.fromCodePoint(value) : match
}).replace(/\s+/g, ' ').trim()

function literal(node) {
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) return node.text
  if (ts.isArrayLiteralExpression(node)) return node.elements.map(literal)
  if (ts.isObjectLiteralExpression(node)) return Object.fromEntries(node.properties.filter(ts.isPropertyAssignment).map((property) => [property.name.getText().replace(/^['"]|['"]$/g, ''), literal(property.initializer)]))
  return undefined
}

const navSource = ts.createSourceFile('nav.ts', await readFile(path.join(root, 'src/components/nav.ts'), 'utf8'), ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
let navigation = []
function readNavigation(node) {
  if (ts.isVariableDeclaration(node) && node.name.getText() === 'navStructure' && node.initializer) navigation = literal(node.initializer) ?? []
  ts.forEachChild(node, readNavigation)
}
readNavigation(navSource)
const navByHref = new Map(navigation.flatMap((group) => group.items.map((item) => [item.href, { title: item.title, section: group.section }])))

function textOf(node) {
  if (ts.isJsxText(node)) return node.text
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) return node.text
  if (ts.isJsxExpression(node)) return node.expression ? textOf(node.expression) : ''
  if (ts.isJsxElement(node) || ts.isJsxFragment(node)) return node.children.map(textOf).join('')
  if (ts.isTemplateExpression(node)) return node.head.text + node.templateSpans.map((span) => textOf(span.expression) + span.literal.text).join('')
  return ''
}

const index = []
for (const page of await loadPages()) {
  const { file } = page
  const relative = path.relative(appRoot, path.dirname(file)).split(path.sep).filter((part) => part && !part.startsWith('(')).join('/')
  const href = relative ? `/${relative}` : '/'
  const source = ts.createSourceFile(file, await readFile(file, 'utf8'), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX)
  const metadata = navByHref.get(href)
  let pageTitle = page.title
  const section = metadata?.section ?? 'Documentation'
  const segments = [{ title: pageTitle, href, heading: false, content: [page.description] }]
  const slugs = new Map()
  let current = segments[0]

  function visit(node) {
    if (ts.isJsxElement(node)) {
      const tag = node.openingElement.tagName.getText(source)
      if (['script', 'style', 'svg', 'nav', 'footer'].includes(tag)) return
      if (tag === 'h1') {
        pageTitle = clean(textOf(node)) || pageTitle
        segments[0].title = pageTitle
        return
      }
      if (/^h[2-4]$/.test(tag) && href !== '/') {
        const title = clean(textOf(node))
        const idAttribute = node.openingElement.attributes.properties.find((attribute) => ts.isJsxAttribute(attribute) && attribute.name.getText(source) === 'id')
        const explicitId = idAttribute?.initializer && clean(textOf(idAttribute.initializer))
        const base = title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'section'
        const count = (slugs.get(base) ?? 0) + 1
        slugs.set(base, count)
        const slug = explicitId || (count === 1 ? base : `${base}-${count}`)
        current = { title, href: `${href}#${slug}`, heading: true, content: [] }
        segments.push(current)
        return
      }
      node.children.forEach(visit)
      return
    }
    if (ts.isJsxText(node) || ts.isJsxExpression(node)) {
      const text = clean(textOf(node))
      if (text) current.content.push(text)
      return
    }
    ts.forEachChild(node, visit)
  }
  let contentRoot
  function findContent(node) {
    if (ts.isJsxElement(node) && ['DocsLayout', 'main'].includes(node.openingElement.tagName.getText(source))) contentRoot ??= node
    ts.forEachChild(node, findContent)
  }
  findContent(source)
  visit(contentRoot)
  for (const segment of segments) index.push({ title: segment.title, pageTitle, href: segment.href, section, content: clean(segment.content.join(' ')), heading: segment.heading })
}

await mkdir(path.join(root, 'public'), { recursive: true })
await writeFile(path.join(root, 'public/search-index.json'), `${JSON.stringify(index)}\n`)
console.log(`Indexed ${index.filter((entry) => !entry.heading).length} pages and ${index.filter((entry) => entry.heading).length} heading sections.`)
