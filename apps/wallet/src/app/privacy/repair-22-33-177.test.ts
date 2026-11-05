import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { expect, test } from 'vitest';
import PrivacyPage from './page';
import './page.test';

function privacy_recipient_drift_discloses_current_recipients_in_the_rendered_page() {
  const html = renderToStaticMarkup(createElement(PrivacyPage));
  const visibleText = (markup: string) =>
    markup
      .replace(/<[^>]+>/g, '')
      .replace(/\s+/g, ' ')
      .trim();
  const disclosures = html
    .match(/<section>[\s\S]*?<\/section>/g)
    ?.find((section) => visibleText(section).includes('2. Data We Collect'));
  expect(disclosures).toBeDefined();
  const text = visibleText(disclosures!);

  expect(text).toContain('configured Paxeer Network chain RPC at its root endpoint');
  expect(text).toContain(
    'If you select a custom RPC or another network in wallet settings, that endpoint receives those requests.'
  );
  expect(text).toContain(
    'Your public wallet address is sent to the wallet server through a same-origin proxy'
  );
  expect(text).toContain(
    'The server forwards these queries to the configured Paxeer indexer service (default: api.paxscan.io).'
  );
  expect(text).toContain('configured explorer (default: paxscan.io)');
  expect(text).toContain('using the Blockscout API');
  expect(text).toContain('Sign-in goes through Supabase');
  expect(text).toContain('record persists on the server');
  expect(text).toContain('sent through the wallet server to OpenAI at api.openai.com');

  const rows = [...html.matchAll(/<tr\b[^>]*>([\s\S]*?)<\/tr>/g)].map((match) =>
    [...match[1].matchAll(/<td\b[^>]*>([\s\S]*?)<\/td>/g)].map((cell) => visibleText(cell[1]))
  );
  expect(rows).toContainEqual([
    'Configured chain RPC or selected network endpoint',
    'Blockchain RPC',
    'Public address, signed transactions',
  ]);
  expect(rows).toContainEqual([
    'Configured explorer (default: paxscan.io)',
    'Explorer links',
    'Public address, transaction hashes',
  ]);
  expect(rows).toContainEqual([
    'Wallet server and configured Paxeer indexer (default: api.paxscan.io)',
    'Portfolio, price and in-app explorer data via the same-origin Blockscout proxy',
    'Public address, token addresses, transaction hashes',
  ]);
  expect(rows).toContainEqual([
    'Supabase',
    'Sign-in and authentication sessions',
    'Authentication information you submit and session data',
  ]);
  expect(rows).toContainEqual([
    'Wallet push server',
    'Push subscription storage and notification delivery',
    'Wallet address, push endpoint, subscription keys, timestamps, optional user agent and tags',
  ]);
  expect(rows).toContainEqual([
    'OpenAI (api.openai.com)',
    'Assistant chat responses',
    'Chat messages and whether a wallet is connected',
  ]);

  for (const retiredRecipient of [
    'public-rpc.paxeer.app',
    'paxscan.paxeer.app',
    'us-east-1.user-stats.sidiora.exchange',
  ]) {
    expect(html).not.toContain(retiredRecipient);
  }
}

test(
  privacy_recipient_drift_discloses_current_recipients_in_the_rendered_page,
  privacy_recipient_drift_discloses_current_recipients_in_the_rendered_page
);
