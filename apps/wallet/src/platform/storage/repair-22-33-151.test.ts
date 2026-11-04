// @vitest-environment jsdom
// @vitest-environment-options {"storageQuota":1000000}

import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { recoverTransferObservation, submittedTransfer } from '@paxeer/wallet';
import type { ContactRecord, JsonValue } from './repositories';

interface StorageBrowser {
  reconfigure(options: { url: string }): void;
  window: Pick<Window, 'localStorage' | 'sessionStorage' | 'close'>;
}

declare const jsdom: StorageBrowser & {
  constructor: new (html: string, options: { url: string }) => StorageBrowser;
};

let storageWindow = jsdom.window;

const ORIGIN = 'http://localhost:3000/';
const BROWSER_QUOTA = 1_000_000;
const FILLER_KEY = 'repair-22-33-151:space';
const SENDER = '0x1111111111111111111111111111111111111111';
const RECIPIENT = '0x2222222222222222222222222222222222222222';
const HASH = `0x${'ab'.repeat(32)}`;
const STORAGE_AREAS = ['localStorage', 'sessionStorage'] as const;
const originalStorageDescriptors = new Map(
  STORAGE_AREAS.map((area) => [area, Object.getOwnPropertyDescriptor(globalThis, area)])
);

beforeEach(() => {
  jsdom.reconfigure({ url: ORIGIN });
  storageWindow = jsdom.window;
  for (const area of STORAGE_AREAS) {
    Object.defineProperty(globalThis, area, {
      configurable: true,
      get: () => storageWindow[area],
    });
  }
  localStorage.clear();
  sessionStorage.clear();
  vi.resetModules();
});

afterEach(() => {
  jsdom.reconfigure({ url: ORIGIN });
  storageWindow = jsdom.window;
  localStorage.clear();
  sessionStorage.clear();
  for (const area of STORAGE_AREAS) {
    const descriptor = originalStorageDescriptors.get(area);
    if (descriptor) Object.defineProperty(globalThis, area, descriptor);
    else Reflect.deleteProperty(globalThis, area);
  }
});

function exhaustBrowserQuota(storage: Storage): void {
  let used = 0;
  for (let index = 0; index < storage.length; index += 1) {
    const key = storage.key(index);
    if (key !== null) used += key.length + (storage.getItem(key)?.length ?? 0);
  }
  storage.setItem(FILLER_KEY, 'x'.repeat(BROWSER_QUOTA - used - FILLER_KEY.length));
  expect(() => storage.setItem('overflow', 'x')).toThrow(/quota/i);
}

async function reopenRepositories() {
  vi.resetModules();
  return import('./repositories');
}

async function produceContact(): Promise<ContactRecord> {
  const { addContact } = await import('@/lib/contacts');
  return addContact('Treasury', RECIPIENT, 'Saved through the contact producer');
}

async function migration_setItem_refusal_survives_reopen_and_retry() {
  const contact = await produceContact();
  let repositories = await import('./repositories');
  const key = repositories.contactsRepository.registration.key;
  const legacy = JSON.stringify(repositories.contactsRepository.read());
  localStorage.setItem('paxeer_contacts', legacy);
  localStorage.removeItem(key);
  exhaustBrowserQuota(localStorage);

  for (let attempt = 0; attempt < 2; attempt += 1) {
    expect(() => repositories.contactsRepository.read()).toThrow(
      expect.objectContaining({
        name: 'StoragePersistenceError',
        code: 'STORAGE_WRITE_FAILED',
        cause: expect.objectContaining({ name: 'QuotaExceededError' }),
      })
    );
    expect(localStorage.getItem('paxeer_contacts')).toBe(legacy);
    expect(localStorage.getItem(key)).toBeNull();
    repositories = await reopenRepositories();
  }

  localStorage.removeItem(FILLER_KEY);
  const { loadContacts } = await import('@/lib/contacts');
  expect(loadContacts()).toEqual([contact]);
  expect(localStorage.getItem('paxeer_contacts')).toBeNull();
  const durable = localStorage.getItem(key);
  expect(durable).not.toBeNull();
  repositories = await reopenRepositories();
  expect(repositories.contactsRepository.read()).toEqual([contact]);
  expect(localStorage.getItem(key)).toBe(durable);
}

test(
  migration_setItem_refusal_survives_reopen_and_retry.name,
  migration_setItem_refusal_survives_reopen_and_retry
);

async function migration_security_choice_refuses_without_deleting_source_until_retry() {
  let repositories = await import('./repositories');
  localStorage.setItem('paxeer:wallet-kind', 'injected');
  exhaustBrowserQuota(localStorage);
  for (let attempt = 0; attempt < 2; attempt += 1) {
    expect(() => repositories.custodyChoiceRepository.read()).toThrow(
      expect.objectContaining({ name: 'StoragePersistenceError', code: 'STORAGE_WRITE_FAILED' })
    );
    expect(localStorage.getItem('paxeer:wallet-kind')).toBe('injected');
    expect(localStorage.getItem(repositories.custodyChoiceRepository.registration.key)).toBeNull();
    repositories = await reopenRepositories();
  }
  localStorage.removeItem(FILLER_KEY);
  expect(repositories.custodyChoiceRepository.read()).toBe('injected');
  expect(localStorage.getItem('paxeer:wallet-kind')).toBeNull();
  repositories = await reopenRepositories();
  expect(repositories.custodyChoiceRepository.read()).toBe('injected');
}

test(
  migration_security_choice_refuses_without_deleting_source_until_retry.name,
  migration_security_choice_refuses_without_deleting_source_until_retry
);

async function migration_record_quota_preserves_source_across_reopen() {
  const contact = await produceContact();
  let repositories = await import('./repositories');
  const key = repositories.contactsRepository.registration.key;
  const contacts = Array.from({ length: 500 }, (_, index) => ({
    ...contact,
    id: `contact-${index}`,
    address: `0x${(index + 1).toString(16).padStart(40, '0')}`,
    name: 'N'.repeat(80),
    note: 'é'.repeat(280),
  }));
  expect(() => repositories.contactsRepository.write(contacts)).toThrow(
    expect.objectContaining({ code: 'STORAGE_QUOTA_EXCEEDED' })
  );
  expect(repositories.contactsRepository.read()).toEqual([contact]);
  const legacy = JSON.stringify(contacts);
  localStorage.setItem('paxeer_contacts', legacy);
  localStorage.removeItem(key);

  for (let attempt = 0; attempt < 2; attempt += 1) {
    expect(() => repositories.contactsRepository.read()).toThrow(
      expect.objectContaining({ name: 'StoragePersistenceError', code: 'STORAGE_QUOTA_EXCEEDED' })
    );
    expect(localStorage.getItem('paxeer_contacts')).toBe(legacy);
    expect(localStorage.getItem(key)).toBeNull();
    repositories = await reopenRepositories();
  }
}

test(
  migration_record_quota_preserves_source_across_reopen.name,
  migration_record_quota_preserves_source_across_reopen
);

async function migration_validation_refusal_preserves_source_across_reopen() {
  const malformed = '[{"name":"incomplete contact"}]';
  localStorage.setItem('paxeer_contacts', malformed);
  let repositories = await import('./repositories');
  for (let attempt = 0; attempt < 2; attempt += 1) {
    expect(repositories.contactsRepository.read()).toEqual([]);
    expect(localStorage.getItem('paxeer_contacts')).toBe(malformed);
    expect(localStorage.getItem(repositories.contactsRepository.registration.key)).toBeNull();
    const { getBackgroundFailures } = await import('@/platform/status/background-failures');
    expect(getBackgroundFailures()[0]).toMatchObject({ code: 'STORAGE_MIGRATION_FAILED' });
    repositories = await reopenRepositories();
  }
}

test(
  migration_validation_refusal_preserves_source_across_reopen.name,
  migration_validation_refusal_preserves_source_across_reopen
);

async function storage_unavailable_refuses_write_update_and_contact_success_across_reopen() {
  let repositories = await import('./repositories');
  expect(repositories.custodyChoiceRepository.write('embedded')).toEqual({
    persisted: true,
    value: 'embedded',
  });
  const key = repositories.custodyChoiceRepository.registration.key;
  const durable = localStorage.getItem(key);

  const unavailableBrowser = new jsdom.constructor('', { url: 'about:blank' });
  storageWindow = unavailableBrowser.window;
  try {
    for (let attempt = 0; attempt < 2; attempt += 1) {
      expect(() => localStorage).toThrow(/opaque origins/i);
      expect(() => repositories.custodyChoiceRepository.write('injected')).toThrow(
        expect.objectContaining({ name: 'StoragePersistenceError', code: 'STORAGE_UNAVAILABLE' })
      );
      expect(() => repositories.custodyChoiceRepository.update(() => 'injected')).toThrow(
        expect.objectContaining({ name: 'StoragePersistenceError', code: 'STORAGE_UNAVAILABLE' })
      );
      const { addContact } = await import('@/lib/contacts');
      expect(() => addContact('Treasury', RECIPIENT)).toThrow(
        expect.objectContaining({ code: 'STORAGE_UNAVAILABLE' })
      );
      repositories = await reopenRepositories();
    }
  } finally {
    storageWindow = jsdom.window;
    unavailableBrowser.window.close();
  }

  expect(localStorage.getItem(key)).toBe(durable);
  expect(repositories.custodyChoiceRepository.read()).toBe('embedded');
  expect(repositories.contactsRepository.read()).toEqual([]);
  expect(repositories.custodyChoiceRepository.update(() => 'injected')).toBe('injected');
  repositories = await reopenRepositories();
  expect(repositories.custodyChoiceRepository.read()).toBe('injected');
}

test(
  storage_unavailable_refuses_write_update_and_contact_success_across_reopen.name,
  storage_unavailable_refuses_write_update_and_contact_success_across_reopen
);

async function storage_quota_refuses_write_update_and_contact_success_across_reopen() {
  const contact = await produceContact();
  let repositories = await import('./repositories');
  repositories.custodyChoiceRepository.write(null);
  const key = repositories.custodyChoiceRepository.registration.key;
  const durable = localStorage.getItem(key);
  exhaustBrowserQuota(localStorage);

  for (let attempt = 0; attempt < 2; attempt += 1) {
    expect(() => repositories.custodyChoiceRepository.write('embedded')).toThrow(
      expect.objectContaining({ code: 'STORAGE_WRITE_FAILED' })
    );
    expect(() => repositories.custodyChoiceRepository.update(() => 'injected')).toThrow(
      expect.objectContaining({ code: 'STORAGE_WRITE_FAILED' })
    );
    const { addContact } = await import('@/lib/contacts');
    expect(() => addContact('Second', SENDER)).toThrow(
      expect.objectContaining({ code: 'STORAGE_WRITE_FAILED' })
    );
    expect(localStorage.getItem(key)).toBe(durable);
    expect(repositories.custodyChoiceRepository.read()).toBeNull();
    expect(repositories.contactsRepository.read()).toEqual([contact]);
    repositories = await reopenRepositories();
  }

  localStorage.removeItem(FILLER_KEY);
  expect(repositories.custodyChoiceRepository.write('embedded')).toEqual({
    persisted: true,
    value: 'embedded',
  });
  const { addContact, loadContacts } = await import('@/lib/contacts');
  const second = addContact('Second', SENDER);
  expect(loadContacts()).toEqual([contact, second]);
  repositories = await reopenRepositories();
  expect(repositories.custodyChoiceRepository.read()).toBe('embedded');
  expect(repositories.contactsRepository.read()).toEqual([contact, second]);
}

test(
  storage_quota_refuses_write_update_and_contact_success_across_reopen.name,
  storage_quota_refuses_write_update_and_contact_success_across_reopen
);

async function storage_record_quota_update_refuses_without_acknowledging_unsaved_state() {
  let repositories = await import('./repositories');
  const saved = repositories.preferencesRepository.read();
  repositories.preferencesRepository.write(saved);
  const tooLarge = {
    ...saved,
    networks: Array.from({ length: 32 }, (_, index) => ({
      chainId: 125000001 + index,
      kind: 'virtual' as const,
      name: `Chain ${index}`,
      rpcUrl: `https://rpc.example/${'a'.repeat(480)}`,
      explorerUrl: `https://explorer.example/${'b'.repeat(480)}`,
    })),
  };
  expect(() => repositories.preferencesRepository.update(() => tooLarge)).toThrow(
    expect.objectContaining({ name: 'StoragePersistenceError', code: 'STORAGE_QUOTA_EXCEEDED' })
  );
  repositories = await reopenRepositories();
  expect(repositories.preferencesRepository.read()).toEqual(saved);
  expect(
    repositories.preferencesRepository.update((current) => ({ ...current, currency: 'EUR' }))
  ).toEqual({ ...saved, currency: 'EUR' });
  repositories = await reopenRepositories();
  expect(repositories.preferencesRepository.read().currency).toBe('EUR');
}

test(
  storage_record_quota_update_refuses_without_acknowledging_unsaved_state.name,
  storage_record_quota_update_refuses_without_acknowledging_unsaved_state
);

async function legacy_transfer_partial_data_survives_quota_reopen_and_retry() {
  const observation = submittedTransfer({
    hash: HASH,
    chainId: 125,
    intent: { sender: SENDER, recipient: RECIPIENT, amountRaw: '1000000000000000000' },
  });
  let repositories = await import('./repositories');
  const cases = [
    {
      name: 'submittedTransfersRepository' as const,
      prefix: 'paxeer.wallet.submittedTransfer:',
      entry: `125:${SENDER}`,
      value: {
        ...observation.identity,
        sender: SENDER,
        recipient: RECIPIENT,
        amount: '1',
        symbol: 'PAX',
        decimals: 18,
      },
    },
    {
      name: 'transferObservationsRepository' as const,
      prefix: 'paxeer.wallet.transferObservation:',
      entry: `125:${HASH}`,
      value: observation,
    },
  ];
  for (const scenario of cases) {
    localStorage.clear();
    const repository = repositories[scenario.name];
    const value = JSON.parse(JSON.stringify(scenario.value)) as JsonValue;
    repository.update((records) => repositories.putTransferRecord(records, scenario.entry, value));
    const legacy = JSON.stringify(repository.read()[scenario.entry]);
    const sourceKey = `${scenario.prefix}${scenario.entry}`;
    const corruptKey = `${scenario.prefix}125:corrupt`;
    localStorage.setItem(sourceKey, legacy);
    localStorage.setItem(corruptKey, '{bad json');
    localStorage.removeItem(repository.registration.key);
    exhaustBrowserQuota(localStorage);

    for (let attempt = 0; attempt < 2; attempt += 1) {
      expect(() => repositories[scenario.name].read()).toThrow(
        expect.objectContaining({ code: 'STORAGE_WRITE_FAILED' })
      );
      expect(localStorage.getItem(sourceKey)).toBe(legacy);
      expect(localStorage.getItem(corruptKey)).toBe('{bad json');
      expect(localStorage.getItem(repository.registration.key)).toBeNull();
      repositories = await reopenRepositories();
    }
    localStorage.removeItem(FILLER_KEY);
    const recovered = repositories[scenario.name].read();
    expect(recovered).toEqual({ [scenario.entry]: value });
    expect(localStorage.getItem(sourceKey)).toBeNull();
    expect(localStorage.getItem(corruptKey)).toBeNull();
    if (scenario.name === 'transferObservationsRepository') {
      expect(recoverTransferObservation(observation.identity, recovered[scenario.entry])).toEqual(
        observation
      );
    } else {
      expect(recovered[scenario.entry]).toMatchObject(observation.identity);
    }
    repositories = await reopenRepositories();
    expect(repositories[scenario.name].read()).toEqual(recovered);
  }
}

test(
  legacy_transfer_partial_data_survives_quota_reopen_and_retry.name,
  legacy_transfer_partial_data_survives_quota_reopen_and_retry
);

async function legacy_metadata_partial_data_survives_quota_reopen_and_retry() {
  const { cacheSet } = await import('@/lib/metaCache');
  cacheSet('token:125', { name: 'Paxeer', symbol: 'PAX', decimals: 18 });
  let repositories = await import('./repositories');
  const entry = repositories.metadataCacheRepository.read()['token:125'];
  const key = repositories.metadataCacheRepository.registration.key;
  const legacy = JSON.stringify(entry);
  localStorage.setItem('pax:meta:token:125', legacy);
  localStorage.setItem('pax:meta:corrupt', '{broken');
  localStorage.setItem('pax:meta:invalid', JSON.stringify({ ts: 'invalid', data: null }));
  localStorage.removeItem(key);
  exhaustBrowserQuota(localStorage);

  for (let attempt = 0; attempt < 2; attempt += 1) {
    expect(() => repositories.metadataCacheRepository.read()).toThrow(
      expect.objectContaining({ code: 'STORAGE_WRITE_FAILED' })
    );
    expect(localStorage.getItem('pax:meta:token:125')).toBe(legacy);
    expect(localStorage.getItem('pax:meta:corrupt')).toBe('{broken');
    expect(localStorage.getItem(key)).toBeNull();
    repositories = await reopenRepositories();
  }
  localStorage.removeItem(FILLER_KEY);
  const { cacheGet } = await import('@/lib/metaCache');
  expect(cacheGet('token:125', 60_000)).toEqual(entry.data);
  expect(localStorage.getItem('pax:meta:token:125')).toBeNull();
  expect(localStorage.getItem('pax:meta:corrupt')).toBeNull();
  expect(localStorage.getItem('pax:meta:invalid')).toBeNull();
  repositories = await reopenRepositories();
  expect(repositories.metadataCacheRepository.read()).toEqual({ 'token:125': entry });
}

test(
  legacy_metadata_partial_data_survives_quota_reopen_and_retry.name,
  legacy_metadata_partial_data_survives_quota_reopen_and_retry
);

async function legacy_session_pending_send_survives_refusal_reopen_and_retry() {
  const event = {
    symbol: 'PAX',
    amount: '1',
    decimals: 18,
    recipient: RECIPIENT,
    txHash: HASH,
    timestamp: Date.now(),
  };
  let optimistic = await import('@/lib/optimistic');
  optimistic.storePendingSend(event);
  let repositories = await import('./repositories');
  const key = repositories.pendingSendRepository.registration.key;
  const legacy = JSON.stringify(repositories.pendingSendRepository.read());
  sessionStorage.setItem('paxeer_pending_send', legacy);
  sessionStorage.removeItem(key);
  exhaustBrowserQuota(sessionStorage);
  expect(() => optimistic.consumePendingSend()).toThrow(
    expect.objectContaining({ code: 'STORAGE_WRITE_FAILED' })
  );
  expect(sessionStorage.getItem('paxeer_pending_send')).toBe(legacy);
  repositories = await reopenRepositories();
  optimistic = await import('@/lib/optimistic');
  expect(() => optimistic.consumePendingSend()).toThrow(
    expect.objectContaining({ code: 'STORAGE_WRITE_FAILED' })
  );
  expect(sessionStorage.getItem('paxeer_pending_send')).toBe(legacy);
  sessionStorage.removeItem(FILLER_KEY);
  expect(optimistic.consumePendingSend()).toEqual(event);
  expect(sessionStorage.getItem('paxeer_pending_send')).toBeNull();
  repositories = await reopenRepositories();
  expect(repositories.pendingSendRepository.read()).toBeNull();
}

test(
  legacy_session_pending_send_survives_refusal_reopen_and_retry.name,
  legacy_session_pending_send_survives_refusal_reopen_and_retry
);
