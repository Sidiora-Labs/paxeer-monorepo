import { DocsAssistant } from "@centra/docs-assistant/react";
import '@/styles/globals.css'
import type { Metadata } from 'next'
import { CodeBlockWrapper } from '@/components/CodeBlockWrapper'
import { SiteHeader } from '@/components/SiteHeader'
import { metadataFor, SITE_NAME, SITE_URL } from '@/lib/seo'

export const metadata: Metadata = {
  ...metadataFor('/'), metadataBase: new URL(SITE_URL), applicationName: SITE_NAME,
  icons: { icon: [
    { url: '/brand/pax-black.svg', type: 'image/svg+xml', media: '(prefers-color-scheme: light)' },
    { url: '/brand/pax-white.svg', type: 'image/svg+xml', media: '(prefers-color-scheme: dark)' },
  ] },
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" className="deepai dark" suppressHydrationWarning>
      <head><script dangerouslySetInnerHTML={{ __html: `try{var t=localStorage.getItem('paxeer-theme')==='light'?'light':'dark';document.documentElement.classList.remove('light','dark');document.documentElement.classList.add(t);document.documentElement.dataset.pendingTheme=t}catch{}` }} /></head>
      <body className="deepai" data-theme="dark" suppressHydrationWarning>
        <script dangerouslySetInnerHTML={{ __html: `document.body.dataset.theme=document.documentElement.dataset.pendingTheme||'dark'` }} />
        <a href="#main-content" className="skip-link">Skip to content</a>
        <SiteHeader />
        {children}
        <DocsAssistant layout="drawer" siteName={SITE_NAME} suggestions={["How do I make my first Paxeer X payment?", "How do agents use identities, budgets, and mandates?", "How do I build with the SDKs, CLI, and platform APIs?", "How do markets, settlement, and interoperability work?"]} />
        <CodeBlockWrapper />
      </body>
    </html>
  )
}
