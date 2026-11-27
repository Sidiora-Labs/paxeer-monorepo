import type { Metadata, Viewport } from 'next';
import { Children, cloneElement, isValidElement, type ReactNode } from 'react';
import localFont from 'next/font/local';
import { launchpadCanvasColors } from './presentation-tokens';
import { deploymentColorMode } from '@/core/app-foundation/color-mode';
import renderRootLayout from './root-layout';
import './globals.css';

const manrope = localFont({
  src: [
    { path: '../../public/fonts/manrope-medium.otf', weight: '500', style: 'normal' },
    { path: '../../public/fonts/manrope-semibold.otf', weight: '600', style: 'normal' },
    { path: '../../public/fonts/manrope-bold.otf', weight: '700', style: 'normal' },
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
  other: { 'color-scheme': deploymentColorMode() === 'device' ? 'light dark' : deploymentColorMode() },
};

export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,
  viewportFit: 'cover',
  themeColor: launchpadCanvasColors[deploymentColorMode() === 'light' ? 'light' : 'dark'],
};

export default async function RootLayout(props: { children: ReactNode }) {
  const layout = await renderRootLayout(props);
  return cloneElement(layout, undefined, Children.map(layout.props.children, child => {
    if (!isValidElement<{ className?: string }>(child) || child.type !== 'body') return child;
    return cloneElement(child, { className: `${manrope.variable} ${child.props.className ?? ''}` });
  }));
}
