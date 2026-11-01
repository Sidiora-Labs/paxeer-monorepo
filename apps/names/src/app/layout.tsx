import type { ReactNode } from 'react';

export const metadata = {
  title: 'Paxeer Names shell',
  description: 'Serve published .pax sites after storage-proof verification.',
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
