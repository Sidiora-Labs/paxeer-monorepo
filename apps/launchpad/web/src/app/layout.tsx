import type { Metadata, Viewport } from 'next';
import type { ReactNode } from 'react';
import { headers } from 'next/headers';
import { BackendConfigurationError } from '@/ui/shared/backend-configuration-error';
import { backendConfigurationProblems } from '@/core/backend-origins';
import './globals.css';

export const metadata: Metadata = {
  title: { default: 'Sidiora Launch', template: '%s | Sidiora Launch' },
  description: 'Launch and trade coins on Paxeer',
  icons: {
    icon: '/Kindle-Launch-logo-dark.png',
    shortcut: '/Kindle-Launch-logo-dark.png',
    apple: '/Kindle-Launch-logo-dark.png',
  },
  manifest: '/manifest.json',
};

export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,
  viewportFit: 'cover',
  themeColor: '#07090D',
};

export default async function RootLayout({ children }: { children: ReactNode }) {
  const NEXT_PUBLIC_PRIVATE_QUALIFICATION_AUTHORITY_PROFILE = process.env.NEXT_PUBLIC_PRIVATE_QUALIFICATION_AUTHORITY_PROFILE;
  const privateFixtureProfileInput = NEXT_PUBLIC_PRIVATE_QUALIFICATION_AUTHORITY_PROFILE;
  // A production build without its backend origins renders one explicit error
  // state and mounts nothing that would contact a backend (no fallback host).
  const configurationProblems = backendConfigurationProblems();
  if (configurationProblems.length > 0) {
    console.error(`[sidiora] backend origins are not configured: ${configurationProblems.map(problem => problem.message).join('; ')}`);
    return (
      <html lang="en">
        <body className="bg-canvas text-text">
          <BackendConfigurationError problems={configurationProblems} />
        </body>
      </html>
    );
  }
  const [
    { cookieStorage, cookieToInitialState, createStorage },
    { createWagmiConfig },
    { default: AppProviders },
    { PRIVATE_FIXTURE_WAGMI_STORAGE_KEY },
    { default: AppShell },
    { default: PlatformRulesModal },
    { default: CookieBanner },
  ] = await Promise.all([
    import('wagmi'),
    import('@/core/wagmi-config'),
    import('@/ui/providers/app-providers'),
    import('@/core/network/deployment'),
    import('@/shell/app-shell'),
    import('@/ui/shared/platform-rules-modal'),
    import('@/ui/shared/cookie-banner'),
  ]);
  const hdrs = await headers();
  const cookie = hdrs.get('cookie') ?? '';
  const config = createWagmiConfig();
  const storage = privateFixtureProfileInput === undefined ? config.storage : createStorage({ key: PRIVATE_FIXTURE_WAGMI_STORAGE_KEY, storage: cookieStorage });
  const wagmiInitialState = cookieToInitialState({ ...config, storage }, cookie);

  return (
    <html lang="en">
      <body className="bg-canvas text-text">
        <PlatformRulesModal />
        <CookieBanner />
        <AppProviders wagmiInitialState={wagmiInitialState} privateFixtureProfileInput={privateFixtureProfileInput}>
          <AppShell>
            {children}
          </AppShell>
        </AppProviders>
      </body>
    </html>
  );
}
