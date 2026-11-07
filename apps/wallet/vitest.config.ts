import { defineConfig } from 'vitest/config';
import path from 'node:path';
import ledgerProof from './src/surfaces/test/fixtures/ledger-proof-v1.json';

const walletSdk = path.resolve(__dirname, '../../sdk/wallet/src/index.ts');
const layerxSdkBrowser = path.resolve(
  __dirname,
  '../../services/agent/sdk/typescript/src/browser.ts'
);

export default defineConfig({
  test: {
    environment: 'node',
    globals: true,
    provide: { ledgerProof },
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
  },
  resolve: {
    alias: {
      '@paxeer/wallet': walletSdk,
      '@sidiora/layerx-sdk/browser': layerxSdkBrowser,
      '@sidiora/layerx-sdk': layerxSdkBrowser,
      '@': path.resolve(__dirname, 'src'),
    },
    dedupe: [
      'viem',
      '@supabase/supabase-js',
      '@noble/curves',
      '@noble/hashes',
      '@scure/bip32',
      '@scure/bip39',
      'ws',
      'react',
      'react-dom',
    ],
  },
});
