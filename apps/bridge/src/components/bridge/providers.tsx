'use client';

import { ThemeProvider } from '@/components/theme-provider';
import { Toaster } from '@/components/ui/sonner';
import { TooltipProvider } from '@/components/ui/tooltip';
import { WalletHydrationProvider } from '@/lib/bridge/use-hydrated-account';
import { wagmiConfig } from '@/lib/bridge/wagmi';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';
import { WagmiProvider } from 'wagmi';

export function Providers({ children }: { children: ReactNode }) {
  const [ queryClient ] = useState(
    () =>
      new QueryClient({
        defaultOptions: { queries: { staleTime: 12_000, retry: 2, refetchOnWindowFocus: false } },
      }),
  );

  return (
    <ThemeProvider>
      <WagmiProvider config={ wagmiConfig }>
        <WalletHydrationProvider>
          <QueryClientProvider client={ queryClient }>
            <TooltipProvider>
              { children }
              <Toaster position="bottom-center"/>
            </TooltipProvider>
          </QueryClientProvider>
        </WalletHydrationProvider>
      </WagmiProvider>
    </ThemeProvider>
  );
}
