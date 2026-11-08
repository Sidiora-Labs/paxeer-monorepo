import { defineStorageNamespace } from './registry';

const SWAP_KEY = /^pax:swap:(?:tokens|operation):v1:/;

function parseSwapRecord(key: string, value: string): void {
  if (!SWAP_KEY.test(key) || key.length > 4_096 || typeof value !== 'string') {
    throw new TypeError('Invalid swap record');
  }
  JSON.parse(value);
}

export const swapRecordsStorage = defineStorageNamespace({
  id: 'swap-records',
  key: 'pax:swap:',
  owner: 'swap',
  schema: 'Version-1 serialized swap records keyed by network and wallet',
  version: 1,
  area: 'local',
  sensitivity: 'security-relevant',
  retention: 'Until explicit reset or uninstall; recovery remains isolated by network and wallet',
  quotaBytes: 2_097_152,
  migration:
    'Validate existing version-1 scoped records in place without changing their native format',
  resetOn: ['reset', 'uninstall'],
  corruption: 'fail-closed',
  prohibitedData: ['mnemonic', 'private key', 'unlock secret', 'authentication credential'],
  maxRecords: 1_000,
  parse: parseSwapRecord,
});

export const swapJournalStorage = defineStorageNamespace({
  id: 'swap-journal',
  key: 'paxeer:swap-journal:v1:',
  owner: 'swap',
  schema: 'Version-1 swap intent journals keyed by chain and wallet',
  version: 1,
  area: 'local',
  sensitivity: 'security-relevant',
  retention: 'Until explicit reset or uninstall; unacknowledged intents remain recoverable',
  quotaBytes: 2_097_152,
  migration: 'Validate existing version-1 journals in place without changing their native format',
  resetOn: ['reset', 'uninstall'],
  corruption: 'fail-closed',
  prohibitedData: ['mnemonic', 'private key', 'unlock secret', 'authentication credential'],
  maxRecords: 1_000,
  parse(key, value) {
    if (!/^paxeer:swap-journal:v1:[1-9]\d*:0x[0-9a-f]{40}$/.test(key)) {
      throw new TypeError('Invalid swap journal key');
    }
    const journal: unknown = JSON.parse(value);
    if (
      typeof journal !== 'object' ||
      journal === null ||
      !('version' in journal) ||
      journal.version !== 1 ||
      !('intents' in journal) ||
      !Array.isArray(journal.intents)
    ) {
      throw new TypeError('Invalid swap journal');
    }
  },
});
