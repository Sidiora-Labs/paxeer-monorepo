import type { Metadata, Viewport } from 'next';
import { cloneElement, type ReactNode } from 'react';
import localFont from 'next/font/local';
import { launchpadCanvasColors } from './presentation-tokens';
import renderRootLayout from './root-layout';
import './globals.css';

const manrope = localFont({
  src: [
    { path: '../../public/fonts/manrope-medium.otf', weight: '500', style: 'normal' },
    { path: '../../public/fonts/manrope-semibold.otf', weight: '600', style: 'normal' },
    { path: '../../public/fonts/manrope-bold.otf', weight: '700', style: 'normal' },
    { path: '../../public/fonts/manrope-extrabold.otf', weight: '800', style: 'normal' },
  ],
  variable: '--sl-font-sans',
  display: 'swap',
  adjustFontFallback: false,
  fallback: ['Manrope Fallback', 'ui-sans-serif', 'system-ui', 'sans-serif'],
});

export const metadata: Metadata = {
  title: { default: 'Sidiora Launch', template: '%s | Sidiora Launch' },
  description: 'Launch and trade coins on Paxeer',
  icons: {
    icon: '/Kindle-Launch-logo-dark.png',
    shortcut: '/Kindle-Launch-logo-dark.png',
    apple: '/Kindle-Launch-logo-dark.png',
  },
  manifest: '/manifest.json',
  other: { 'color-scheme': 'light dark' },
};

export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,
  viewportFit: 'cover',
  themeColor: [
    { media: '(prefers-color-scheme: light)', color: launchpadCanvasColors.light },
    { media: '(prefers-color-scheme: dark)', color: launchpadCanvasColors.dark },
  ],
};

export default async function RootLayout(props: { children: ReactNode }) {
  const layout = await renderRootLayout(props);
  const body = layout.props.children;
  return cloneElement(layout, undefined, cloneElement(body, {
    className: `${manrope.variable} ${body.props.className}`,
  }));
}
