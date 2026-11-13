import type { Metadata } from 'next'
import pages from './seo-pages.json'

export const SITE_URL = new URL(process.env.NEXT_PUBLIC_SITE_URL || 'https://docs.paxeer.app').origin
export const SITE_NAME = 'Paxeer X Docs'
export function pageInfo(route: string) {
  const key = (route.replace(/\/$/,'') || '/') as keyof typeof pages
  return { ...(pages[key] || pages['/']), url: SITE_URL + (key === '/' ? '/' : key + '/') }
}
export function metadataFor(route: string): Metadata {
  const page = pageInfo(route)
  const title = route === '/' ? page.title : `${page.title} | ${SITE_NAME}`
  const image = { url: `${SITE_URL}/social/paxeer-x-docs-preview.png`, width: 1733, height: 907, alt: 'Build on Paxeer X. The complete network. One documentation hub. Fine-line network architecture on a dark background.' }
  return {
    title: { absolute: title }, description: page.description,
    alternates: { canonical: page.url, types: { 'text/markdown': `${SITE_URL}/markdown/${route === '/' ? 'index' : route.slice(1)}.md` } },
    robots: { index: true, follow: true, googleBot: { index: true, follow: true, 'max-image-preview': 'large', 'max-snippet': -1 } },
    openGraph: { title, description: page.description, type: route === '/' ? 'website' : 'article', url: page.url, siteName: SITE_NAME, locale: 'en_US', images: [image] },
    twitter: { card: 'summary_large_image', title, description: page.description, images: [image.url] },
  }
}
export function jsonLd(route: string) {
  const page = pageInfo(route)
  return JSON.stringify({ '@context': 'https://schema.org', '@type': route === '/' ? 'WebSite' : 'TechArticle', name: page.title, headline: page.title, description: page.description, url: page.url, inLanguage: 'en', isPartOf: { '@type': 'WebSite', name: SITE_NAME, url: SITE_URL + '/' } }).replace(/</g,'\\u003c')
}
