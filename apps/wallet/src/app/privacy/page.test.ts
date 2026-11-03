import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test } from 'vitest';

function privacy_page_describes_the_web_wallet() {
  const source = readFileSync(join(__dirname, 'page.tsx'), 'utf8');
  const text = source.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ');

  expect(text).toContain('installable web wallet');
  expect(text).toMatch(/Sign-in goes through Supabase/);
  expect(text).toMatch(/wallet SDK uses an embedded wallet or connects an injected wallet/);
  expect(text).toMatch(
    /wallet server stores a push subscription record containing your wallet address/
  );
  expect(text).toMatch(/record persists on the server/);
  expect(text).toMatch(
    /chat messages are sent through the wallet server to OpenAI at api\.openai\.com/
  );

  const table = source.match(/<table\b[^>]*>[\s\S]*?<\/table>/)?.[0];
  expect(table).toMatch(/<tr>[\s\S]*?Supabase[\s\S]*?Sign-in[\s\S]*?<\/tr>/);
  expect(table).toMatch(
    /<tr>[\s\S]*?OpenAI[\s\S]*?api\.openai\.com[\s\S]*?Chat messages[\s\S]*?<\/tr>/
  );

  for (const obsoleteClaim of [
    'chrome.storage.local',
    'No data is stored on our servers',
    'sidePanel',
    'activeTab',
    'Permissions Justification',
    'browser extension',
    'the Extension',
    'other than the blockchain RPC and indexer APIs',
    'No account required',
  ]) {
    expect(source).not.toContain(obsoleteClaim);
  }
}

test(privacy_page_describes_the_web_wallet, privacy_page_describes_the_web_wallet);
