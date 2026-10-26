import { describe, expect, it } from 'vitest';
import { EnvSchema } from '../src/env.js';

const environment = {
  CORS_ORIGINS: 'https://wallet.example.com',
  SUPABASE_URL: 'https://identity.example.com',
  DATABASE_URL: 'postgres://localhost:5432/wallet',
  WALLET_MASTER_KEY: Buffer.alloc(32, 1).toString('base64'),
};

describe('gateway environment', () => {
  it('sequencerUrlDefaultsToFirstReadUrl', function sequencerUrlDefaultsToFirstReadUrl() {
    const defaults = EnvSchema.parse(environment);
    expect(defaults.RPC_URLS).toEqual(['https://api-mainnet-beta.paxeer.network']);
    expect(defaults.SEQUENCER_RPC_URL).toBe(defaults.RPC_URLS[0]);

    const configured = EnvSchema.parse({
      ...environment,
      RPC_URLS: ' https://read-one.example.com/rpc, http://read-two.example.com/rpc ',
    });
    expect(configured.RPC_URLS).toEqual([
      'https://read-one.example.com/rpc',
      'http://read-two.example.com/rpc',
    ]);
    expect(configured.SEQUENCER_RPC_URL).toBe('https://read-one.example.com/rpc');
  });

  it('configuredSequencerUrlIsKept', function configuredSequencerUrlIsKept() {
    for (const url of [
      'https://sequencer.example.com:8443/rpc?chain=125',
      'http://sequencer.example.com:8787/rpc',
    ]) {
      const configured = EnvSchema.parse({
        ...environment,
        RPC_URLS: 'https://read.example.com/rpc',
        SEQUENCER_RPC_URL: url,
      });
      expect(configured.SEQUENCER_RPC_URL).toBe(url);
      expect(configured.RPC_URLS).toEqual(['https://read.example.com/rpc']);
    }

    for (const url of [
      'ftp://sequencer.example.com/rpc',
      'ws://sequencer.example.com/rpc',
      'not-a-url',
      '/rpc',
      'https://',
      '',
      'https://sequencer.example.com/rpc https://other.example.com/rpc',
    ]) {
      expect(() => EnvSchema.parse({ ...environment, SEQUENCER_RPC_URL: url })).toThrow(
        'SEQUENCER_RPC_URL must be an http(s) URL'
      );
    }
  });
});
