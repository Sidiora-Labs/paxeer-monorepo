import { DocsAssistant } from "@centra/docs-assistant/react";
import '@/styles/globals.css'
import type { Metadata } from 'next'
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
    <html lang="en" className="deepai" suppressHydrationWarning>
      <head>
        <meta name="color-scheme" content="light dark" />
        <meta name="text-scale" content="scale" />
        <script dangerouslySetInnerHTML={{ __html: `try{var v=localStorage.getItem('paxeer-theme');var o=v==='light'||v==='dark'?v:null;document.querySelector('meta[name="color-scheme"]').content=o||'light dark';var t=o||(matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light');document.documentElement.classList.add(t)}catch{}` }} />
      </head>
      <body className="deepai" suppressHydrationWarning>
        <script dangerouslySetInnerHTML={{ __html: `document.body.dataset.theme=document.documentElement.classList.contains('dark')?'dark':document.documentElement.classList.contains('light')?'light':matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light'` }} />
        <a href="#main-content" className="skip-link">Skip to content</a>
        <SiteHeader />
        {children}
        <DocsAssistant layout="drawer" siteName={SITE_NAME} suggestions={["How do I make my first Paxeer X payment?", "How do agents use identities, budgets, and mandates?", "How do I build with the SDKs, CLI, and platform APIs?", "How do markets, settlement, and interoperability work?"]} />
      </body>
    </html>
  )
}
