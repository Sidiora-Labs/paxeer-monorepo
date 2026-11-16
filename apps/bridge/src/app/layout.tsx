import { Providers } from '@/components/bridge/providers';
import type { Metadata, Viewport } from 'next';
import { Geist, Geist_Mono } from 'next/font/google';
import { Suspense, type JSX } from 'react';

import { THEME_INIT } from './theme-init';
import './globals.css';

const geistSans = Geist({
  variable: '--font-geist-sans',
  subsets: [ 'latin' ],
  display: 'swap',
});

const geistMono = Geist_Mono({
  variable: '--font-geist-mono',
  subsets: [ 'latin' ],
  display: 'swap',
});

export const metadata: Metadata = {
  title: 'Paxeer Bridge — move assets to and from Paxeer',
  description:
    'The attested PaxeerX bridge: lock assets on Ethereum, Base, Arbitrum, Optimism, BNB Chain, Polygon, Avalanche, HyperEVM or Solana and receive them on Paxeer — and back.',
  applicationName: 'Paxeer Bridge',
};

export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,

};

export default function RootLayout({ children }: LayoutProps<'/'>): JSX.Element {
  return (
    <html lang="en" className={ `${ geistSans.variable } ${ geistMono.variable } h-full antialiased` } suppressHydrationWarning>
      <head>
        <meta name="color-scheme" content="light dark"/>
        <meta name="theme-color"/>
        <meta name="text-scale" content="scale"/>
        <script dangerouslySetInnerHTML={{ __html: THEME_INIT }}/>
      </head>
      <body className="flex min-h-full flex-col bg-background text-foreground">
        <a
          href="#main-content"
          className="skip-link"
        >
          Skip to the bridge
        </a>
        <Suspense fallback={ <p role="status" className="p-6 text-center">Loading bridge…</p> }>
          <Providers>{ children }</Providers>
        </Suspense>
      </body>
    </html>
  );
}
