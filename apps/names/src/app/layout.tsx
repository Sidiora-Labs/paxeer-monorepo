import type { ReactNode } from 'react';
import './globals.css';

export const metadata = {
  title: 'Paxeer Names shell',
  description: 'Serve published .pax sites after storage-proof verification.',
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body>{children}</body>
    </html>
  );
}
