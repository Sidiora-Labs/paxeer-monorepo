import { mkdir, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { loadPages, root } from './docs-content.mjs'

const origin = new URL(process.env.NEXT_PUBLIC_SITE_URL || 'https://docs.paxeer.app').origin
const pages = await loadPages()
await mkdir(path.join(root,'.generated'),{recursive:true})
const canonical = route => origin + (route === '/' ? '/' : route + '/')
const absoluteLinks = body => body.replace(/\]\((\/(?!\/)[^)]+)\)/g, (_, href) => `](${new URL(href, origin).href})`)
const escape = value => value.replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;')
const publicDir = path.join(root,'public')
await mkdir(path.join(publicDir,'markdown'),{recursive:true})
await mkdir(path.join(root,'src/lib'),{recursive:true})
const manifest = Object.fromEntries(pages.map(({route,title,description}) => [route,{title,description}]))
await writeFile(path.join(root,'src/lib/seo-pages.json'),JSON.stringify(manifest,null,2)+'\n')
const index = ['# Paxeer X Documentation','','> Guides and reference for the Paxeer X network, LayerX economy, applications, agents and operations.','','Paxeer X connects EVM applications with LayerX identities, payments, markets, businesses, deterministic programs and verifiable settlement. PAX is the native gas token. Explore the complete network through architecture, integration guides and operational references.','','## Documentation','',...pages.map(page => `- [${page.title}](${canonical(page.route)}): ${page.description}`),'','## Machine-readable pages','',`Each documentation page is available as Markdown under ${origin}/markdown/<route>.md.`, `The complete documentation is at ${origin}/llms-full.txt.`,''].join('\n')
const full = [index,'---','',...pages.map(page=>`URL: ${canonical(page.route)}\n\n${absoluteLinks(page.body)}\n---\n`)].join('\n')
await writeFile(path.join(publicDir,'llms.txt'),index)
await writeFile(path.join(publicDir,'llms-full.txt'),full)
for(const page of pages){
 const target=path.join(publicDir,'markdown',page.route==='/'?'index.md':page.route.slice(1)+'.md')
 await mkdir(path.dirname(target),{recursive:true})
 const body=absoluteLinks(page.body)
 await writeFile(target,`<!-- Source: ${canonical(page.route)} -->\n\n${body}`)
}
await writeFile(path.join(publicDir,'robots.txt'),`User-agent: *\nAllow: /\n\nSitemap: ${origin}/sitemap.xml\n`)
await writeFile(path.join(publicDir,'sitemap.xml'),`<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${pages.map(page=>`  <url><loc>${escape(canonical(page.route))}</loc></url>`).join('\n')}\n</urlset>\n`)
console.log(`Generated metadata, sitemap, robots and LLM documentation for ${pages.length} pages at ${origin}`)

await writeFile(path.join(root,'.generated/assistant-docs.json'),JSON.stringify({name:'Paxeer X Docs',origin,pages:pages.map(page=>({title:page.title,path:page.route==='/'?'/':page.route+'/',content:page.body,headings:page.headings}))}))
