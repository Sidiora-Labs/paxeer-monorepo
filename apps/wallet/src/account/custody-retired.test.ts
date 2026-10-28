import { existsSync, readFileSync, readdirSync } from 'node:fs';
import path from 'node:path';
import { expect, test } from 'vitest';

function accountSourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const file = path.join(directory, entry.name);
    if (entry.isDirectory()) return accountSourceFiles(file);
    return /\.tsx?$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name) ? [file] : [];
  });
}

function account_module_has_no_custody_deposit() {
  for (const file of [
    'DepositFlow.tsx',
    'DepositFlow.test.tsx',
    'custody.ts',
    'test/fixtures/deposit.json',
    'test/fixtures/deposit-explorer-status.json',
  ]) {
    expect(existsSync(path.join(__dirname, file)), `${file} must remain retired`).toBe(false);
  }

  const sources = accountSourceFiles(__dirname);
  expect(sources.length).toBeGreaterThan(0);
  for (const file of sources) {
    const source = readFileSync(file, 'utf8');
    expect(source, path.relative(__dirname, file)).not.toMatch(
      /\b(?:DepositFlow|CUSTODY_PRECOMPILE|CUSTODY_HANDOFF_DOMAIN|CUSTODY_ABI|CustodyDepositError|beneficiaryOf|depositTokenCalldata|CustodyCall|custodyCall|custodyHandoff|prepareCustody)\b|\b0x0*1013\b|LX:CUSTODY:v1|paxeer:custody:v2|paxeer_prepareCustody/i
    );
  }
}

test(account_module_has_no_custody_deposit);
