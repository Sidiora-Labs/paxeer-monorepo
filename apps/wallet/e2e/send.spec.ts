import { readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { expect, test, type Page, type Locator, type TestInfo } from '@playwright/test';
import { formatEther } from 'ethers';
import type { TransferObservation } from '@paxeer/wallet';
import { readSequencerReceipt } from '../../../sdk/wallet/src/receipt';
import { readExplorerStatus } from '../../../sdk/wallet/src/ladder';
import { createWalletClient, http, type Hex } from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { bottomTab, signIn } from './wallet';
import { APP_ENV } from './environment';
import { waitForIndexedCommit } from './explorer-proof';

test.describe('send', () => {
  test.beforeEach(async ({ page }) => {
    await signIn(page);
  });

  test('opens the send form from the wallet home and returns', async ({ page }) => {
    await page.getByRole('button', { name: 'Send', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Send' })).toBeVisible();
    await expect(bottomTab(page, 'Settings')).toHaveCount(0);
    await page.getByRole('button', { name: 'Back' }).click();
    await expect(bottomTab(page, 'Settings')).toBeVisible();
  });

  test('opens the send form from its route', async ({ page }) => {
    await page.goto('./?screen=send');
    await expect(page.getByRole('heading', { name: 'Send' })).toBeVisible();
  });
});

const ANVIL_URL = process.env.WALLET_TRANSFER_TRUTH_ANVIL_URL ?? '';
const ANVIL_SENDER = process.env.WALLET_TRANSFER_TRUTH_SENDER ?? '';
const SEQUENCER_RPC = process.env.WALLET_TRANSFER_TRUTH_SEQUENCER_RPC_URL ?? '';
const OBSERVED_METHODS = new Set([
  'eth_chainId',
  'eth_getTransactionReceipt',
  'eth_getTransactionByHash',
  'eth_getTransactionCount',
  'eth_getBlockByNumber',
  'pax_sequencerReceipt',
]);
const WRONG_RPC = process.env.WALLET_TRANSFER_TRUTH_WRONG_RPC_URL ?? '';
const WALLET_RPC =
  process.env.WALLET_TRANSFER_TRUTH_BROWSER_RPC_URL ?? APP_ENV.NEXT_PUBLIC_PAXEER_RPC_URL;
let observationRpc = ANVIL_URL;
const RECIPIENT = '0x00000000000000000000000000000000000000b1';
const REVERTER = '0x00000000000000000000000000000000000000fe';

async function rpcAt(url: string, method: string, params: readonly unknown[] = []): Promise<any> {
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }),
  });
  const body = (await response.json()) as { result?: unknown; error?: { message: string } };
  if (body.error) throw new Error(`${method}: ${body.error.message}`);
  return body.result;
}

async function anvil(method: string, params: readonly unknown[] = []): Promise<any> {
  return rpcAt(ANVIL_URL, method, params);
}

async function submit(tx: Record<string, string>): Promise<string> {
  return anvil('eth_sendTransaction', [
    { from: ANVIL_SENDER, to: RECIPIENT, value: '0x1', gas: '0x186a0', ...tx },
  ]);
}

async function observe(page: Page, hash: string, rpcUrl = observationRpc): Promise<void> {
  let transaction = await rpcAt(rpcUrl, 'eth_getTransactionByHash', [hash]);
  await expect
    .poll(
      async () => {
        transaction = await rpcAt(rpcUrl, 'eth_getTransactionByHash', [hash]);
        return transaction;
      },
      { intervals: [50], timeout: 60_000 }
    )
    .not.toBeNull();
  if (
    !transaction ||
    transaction.hash.toLowerCase() !== hash.toLowerCase() ||
    transaction.from.toLowerCase() !== ANVIL_SENDER.toLowerCase() ||
    typeof transaction.to !== 'string'
  ) {
    throw new Error('Observed transaction must belong to the authenticated controlled wallet');
  }
  await page.evaluate(
    ({ hash: h, sender, recipient, amount, amountRaw }) => {
      window.localStorage.setItem(
        `paxeer.wallet.submittedTransfer:125:${sender.toLowerCase()}`,
        JSON.stringify({
          hash: h,
          chainId: 125,
          sender,
          symbol: 'PAX',
          decimals: 18,
          recipient,
          amount,
          intent: { sender, recipient, amountRaw },
        })
      );
    },
    {
      hash,
      sender: ANVIL_SENDER,
      recipient: transaction.to,
      amount: formatEther(BigInt(transaction.value)),
      amountRaw: BigInt(transaction.value).toString(),
    }
  );
  await page.goto('./?screen=send');
}

const title = (page: Page) => page.locator('h2[data-truth]');

async function routeObservations(page: Page): Promise<void> {
  await page.route(WALLET_RPC, async (route) => {
    const call = route.request().postDataJSON() as { method?: string } | null;
    if (!call || !OBSERVED_METHODS.has(call.method ?? '')) return route.continue();
    const url = call.method === 'pax_sequencerReceipt' ? SEQUENCER_RPC : observationRpc;
    if (!url) throw new Error('The observed RPC method requires its controlled node URL');
    const response = await route.fetch({ url });
    await route.fulfill({ response });
  });
}

test(
  send_shows_receipted_from_the_sequencer_receipt_and_keeps_it_on_reload.name,
  send_shows_receipted_from_the_sequencer_receipt_and_keeps_it_on_reload
);

test(
  send_reaches_committed_only_with_a_matching_explorer_answer.name,
  send_reaches_committed_only_with_a_matching_explorer_answer
);

test(
  send_observes_the_configured_explorer_commit_reload_and_refusal.name,
  send_observes_the_configured_explorer_commit_reload_and_refusal
);

async function send_observes_the_configured_explorer_commit_reload_and_refusal({
  page,
  baseURL,
}: {
  page: Page;
  baseURL: string | undefined;
}): Promise<void> {
  const key = process.env.WALLET_TRANSFER_TRUTH_SEQUENCER_KEY;
  if (!baseURL || !SEQUENCER_RPC || !key || !/^0x[0-9a-fA-F]{64}$/.test(key))
    throw new Error('The configured app, chain RPC and funded signing key are required');
  const account = privateKeyToAccount(key as Hex);
  expect(account.address.toLowerCase()).toBe(ANVIL_SENDER.toLowerCase());
  expect(await rpcAt(SEQUENCER_RPC, 'eth_chainId')).toBe('0x7d');
  const wallet = createWalletClient({ account, transport: http(SEQUENCER_RPC) });
  await page.exposeBinding(
    'configuredExplorerWalletRequest',
    async (_source, request: { method: string; params?: readonly unknown[] }) => {
      if (request.method === 'eth_accounts' || request.method === 'eth_requestAccounts')
        return wallet.getAddresses();
      return rpcAt(SEQUENCER_RPC, request.method, request.params);
    }
  );
  await page.addInitScript(() => {
    if (!sessionStorage.getItem('configured-explorer-session')) {
      localStorage.removeItem('paxport:v1:custody-choice');
      localStorage.removeItem('paxeer:wallet-kind');
      for (const key of Object.keys(localStorage)) {
        if (/^sb-.*-auth-token$/.test(key)) localStorage.removeItem(key);
      }
      sessionStorage.setItem('configured-explorer-session', 'true');
    }
    const events = new EventTarget();
    const detail = {
      info: {
        uuid: crypto.randomUUID(),
        name: 'Configured explorer signer',
        icon: 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg"/>',
        rdns: 'local.configured.explorer',
      },
      provider: {
        request: (request: { method: string; params?: readonly unknown[] }) =>
          (
            window as unknown as {
              configuredExplorerWalletRequest: (request: {
                method: string;
                params?: readonly unknown[];
              }) => Promise<unknown>;
            }
          ).configuredExplorerWalletRequest(request),
        on: (event: string, listener: EventListener) => events.addEventListener(event, listener),
        removeListener: (event: string, listener: EventListener) =>
          events.removeEventListener(event, listener),
      },
    };
    const announce = () =>
      window.dispatchEvent(new CustomEvent('eip6963:announceProvider', { detail }));
    window.addEventListener('eip6963:requestProvider', announce);
    announce();
  });
  const explorerResponses: { url: string; status: number; body: unknown }[] = [];
  const responseReads: Promise<void>[] = [];
  page.on('response', (response) => {
    if (!/\/api\/v2\/transactions\/0x[0-9a-fA-F]{64}\/status$/.test(response.url())) return;
    responseReads.push(
      response.body().then((body) => {
        explorerResponses.push({
          url: response.url(),
          status: response.status(),
          body: JSON.parse(body.toString()) as unknown,
        });
      })
    );
  });
  await page.goto('./');
  await page.evaluate(() => window.dispatchEvent(new Event('eip6963:requestProvider')));
  await page.getByRole('button', { name: /Configured explorer signer/ }).click();
  await expect(bottomTab(page, 'Settings')).toBeVisible();

  const hash = await wallet.sendTransaction({ chain: null, to: RECIPIENT, value: 1n });
  await observe(page, hash, SEQUENCER_RPC);
  await expect
    .poll(() => explorerResponses.find((response) => response.url.includes(hash))?.url, {
      timeout: 60_000,
    })
    .toBeDefined();
  const statusUrl = explorerResponses.find((response) => response.url.includes(hash))!.url;
  const explorerUrl = statusUrl.slice(0, -`/api/v2/transactions/${hash}/status`.length);
  const committed = await waitForIndexedCommit(
    { rpcUrl: SEQUENCER_RPC, explorerUrl },
    { hash, chainId: 125 }
  );
  const evidence = `committed:${committed.blockHash}`;
  const assertWalletCommit = async () => {
    await expect(title(page)).toHaveText('Transfer Completed');
    await expect(page.locator('li[data-rung="committed"]')).toHaveAttribute(
      'data-source',
      'explorer'
    );
    await expect(page.locator('li[data-rung="committed"]')).toHaveAttribute(
      'data-evidence',
      evidence
    );
    await expect
      .poll(() =>
        page.evaluate((key) => {
          const stored = localStorage.getItem('paxport:v1:transfer-observations');
          return stored ? JSON.parse(stored).value[key] : null;
        }, `125:${hash.toLowerCase()}`)
      )
      .toMatchObject({
        identity: { hash, chainId: 125 },
        blockNumber: committed.blockNumber,
        blockHash: committed.blockHash,
        steps: expect.arrayContaining([{ rung: 'committed', source: 'explorer', state: evidence }]),
      });
  };
  await assertWalletCommit();
  await page.reload();
  await assertWalletCommit();
  await Promise.all(responseReads);
  expect(explorerResponses).toContainEqual({
    url: statusUrl,
    status: 200,
    body: expect.objectContaining({ rung: 'committed', block_number: committed.blockNumber }),
  });

  const missingHash = `0x${'00'.repeat(32)}`;
  const refused = await fetch(`${explorerUrl}/api/v2/transactions/${missingHash}/status`);
  expect(refused.status).toBe(404);
  await expect(readExplorerStatus(explorerUrl, missingHash)).rejects.toThrow(
    'explorer status answered HTTP 404'
  );
  await assertWalletCommit();
}

test.describe('send transfer truth', () => {
  test.beforeAll(async () => {
    if (!ANVIL_URL || !/^0x[0-9a-fA-F]{40}$/.test(ANVIL_SENDER)) {
      throw new Error(
        'WALLET_TRANSFER_TRUTH_ANVIL_URL and WALLET_TRANSFER_TRUTH_SENDER must name a running controlled chain'
      );
    }
    await anvil('evm_setAutomine', [false]);
    await anvil('anvil_setCode', [REVERTER, '0xfe']);
  });

  test.beforeEach(async ({ page }) => {
    observationRpc = ANVIL_URL;
    if (!WALLET_RPC)
      throw new Error(
        'WALLET_TRANSFER_TRUTH_BROWSER_RPC_URL must identify the prebuilt wallet RPC'
      );
    await routeObservations(page);
    await page.goto('./');
    await expect(bottomTab(page, 'Settings')).toBeVisible();
  });

  test('a returned hash is pending until a successful receipt, which is not finality, and reload keeps the evidence', async ({
    page,
  }) => {
    const hash = await submit({});
    await observe(page, hash);
    await expect(title(page)).toHaveAttribute('data-truth', 'pending');
    await page.waitForTimeout(5000);
    await expect(title(page)).toHaveAttribute('data-truth', 'pending');
    await expect(page.getByText('Transfer Completed')).toHaveCount(0);
    await anvil('evm_mine');
    await expect(title(page)).toHaveAttribute('data-truth', 'included');
    await expect(page.locator('li[data-rung="instant"]')).toHaveAttribute('data-source', 'receipt');
    await expect(page.locator('li[data-rung="final"]')).toHaveAttribute('data-reached', 'false');
    await expect(page.getByText('Transfer Completed')).toHaveCount(0);
    await page.reload();
    await expect(title(page)).toHaveAttribute('data-truth', 'included');
    await expect(page.locator('li[data-rung="instant"]')).toHaveAttribute('data-source', 'receipt');
    await expect(page.getByText('Transfer Completed')).toHaveCount(0);
  });

  test('a reverted transfer never displays completion', async ({ page }) => {
    const hash = await submit({ to: REVERTER });
    await anvil('evm_mine');
    await observe(page, hash);
    await expect(title(page)).toHaveAttribute('data-truth', 'reverted');
    await expect(page.getByText('Transfer Reverted')).toBeVisible();
    await expect(page.getByText('Transfer Completed')).toHaveCount(0);
    await expect(page.locator('li[data-rung="instant"]')).toHaveAttribute('data-reached', 'false');
  });

  test('a replaced transfer is distinct from success', async ({ page }) => {
    const nonce = await anvil('eth_getTransactionCount', [ANVIL_SENDER, 'pending']);
    const original = await submit({
      nonce,
      maxFeePerGas: '0x77359400',
      maxPriorityFeePerGas: '0x3b9aca00',
    });
    await observe(page, original);
    await expect(title(page)).toHaveAttribute('data-truth', 'pending');
    await submit({
      nonce,
      value: '0x2',
      maxFeePerGas: '0xee6b2800',
      maxPriorityFeePerGas: '0x77359400',
    });
    await anvil('evm_mine');
    await expect(title(page)).toHaveAttribute('data-truth', 'replaced');
    await expect(page.getByText('Transfer Completed')).toHaveCount(0);
  });

  test('a dropped transfer becomes unknown, and reconnecting does not invent finality', async ({
    page,
  }) => {
    const hash = await submit({});
    await observe(page, hash);
    await expect(title(page)).toHaveAttribute('data-truth', 'pending');
    await anvil('anvil_dropTransaction', [hash]);
    await expect(title(page)).toHaveAttribute('data-truth', 'unknown');
    await page.reload();
    await expect(title(page)).not.toHaveAttribute('data-truth', 'included');
    await expect(page.getByText('Transfer Completed')).toHaveCount(0);
  });

  test('a canonical rollback removes the receipted step and never shows Committed', async ({
    page,
  }) => {
    const snapshot = await anvil('evm_snapshot');
    const hash = await submit({});
    await anvil('evm_mine');
    await observe(page, hash);
    await expect(title(page)).toHaveAttribute('data-truth', 'included');
    const receipt = await readSequencerReceipt(
      (method, params) => rpcAt(SEQUENCER_RPC, method, params),
      hash
    );
    expect(receipt).toMatchObject({ txHash: hash, status: '0x1' });
    expect(receipt?.blockHash).toMatch(/^0x[0-9a-fA-F]{64}$/);
    if (receipt === null || receipt.blockHash === undefined)
      throw new Error('Rollback requires previously verified sequencer receipt evidence');
    await expect(title(page)).toHaveText('Receipted');
    const receipted = page.locator('li[data-rung="receipted"]');
    const committed = page.locator('li[data-rung="committed"]');
    await expect(receipted).toHaveAttribute('data-reached', 'true');
    await expect(receipted).toHaveAttribute('data-source', 'receipt');
    await expect(receipted).toHaveAttribute(
      'data-evidence',
      `receipted:${receipt.blockHash.toLowerCase()}`
    );
    await expect(committed).toHaveAttribute('data-reached', 'false');
    await expect(page.getByText('Transfer Completed')).toHaveCount(0);
    expect(await anvil('evm_revert', [snapshot])).toBe(true);
    await expect(title(page)).toHaveAttribute('data-truth', 'unknown');
    await expect(receipted).toHaveAttribute('data-reached', 'false');
    await expect(receipted).not.toHaveAttribute('data-source');
    await expect(receipted).not.toHaveAttribute('data-evidence');
    await expect(committed).toHaveAttribute('data-reached', 'false');
    await expect(page.getByText('Transfer Completed')).toHaveCount(0);
    await expect(page.locator('[data-retained-evidence]')).toBeVisible();
    const savedObservation = () =>
      page.evaluate((key) => {
        const raw = localStorage.getItem('paxport:v1:transfer-observations');
        if (raw === null) throw new Error('The rolled-back transfer observation was not saved');
        const stored = JSON.parse(raw) as { value: Record<string, TransferObservation> };
        return stored.value[key];
      }, `125:${hash.toLowerCase()}`);
    await expect.poll(savedObservation).toMatchObject({
      truth: 'unknown',
      steps: [],
      blockNumber: null,
      lastVerified: {
        steps: [{ rung: 'receipted', source: 'receipt' }],
        sequencer: receipt,
      },
    });
    await page.reload();
    await expect(title(page)).toHaveAttribute('data-truth', 'unknown');
    await expect(receipted).toHaveAttribute('data-reached', 'false');
    await expect(committed).toHaveAttribute('data-reached', 'false');
    await expect(page.getByText('Transfer Completed')).toHaveCount(0);
    await expect(page.locator('[data-retained-evidence]')).toBeVisible();
  });
});

test('injected chain pinning uses the real official extension and isolated chains', async () => {
  const fs = await import('node:fs/promises');
  const path = await import('node:path');
  const manifestPath = process.env.WALLET_INJECTED_CHAIN_RUNTIME;
  if (!manifestPath) throw new Error('The private source-bound injected chain runtime is required');
  const manifest = JSON.parse(await fs.readFile(manifestPath, 'utf8'));
  if (manifest.schema !== 'paxeer-x.wallet-injected-chain-runtime.v1')
    throw new Error('Invalid injected chain runtime');
  const { chromium } = await import('@playwright/test');
  const context = await chromium.launchPersistentContext(manifest.profile_dir, {
    headless: true,
    channel: 'chromium',
    args: [
      `--disable-extensions-except=${manifest.extension_path}`,
      `--load-extension=${manifest.extension_path}`,
      '--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE localhost, EXCLUDE 127.0.0.1',
    ],
  });
  const cases: Record<string, unknown> = {};
  try {
    const worker =
      context.serviceWorkers().find((value) => value.url().startsWith('chrome-extension://')) ??
      (await context.waitForEvent('serviceworker'));
    const extensionId = new URL(worker.url()).hostname;
    expect(extensionId).toMatch(/^[a-p]{32}$/);
    const home = await context.newPage();
    await home.goto(`chrome-extension://${extensionId}/home.html`);
    const password = (await fs.readFile(manifest.password_file, 'utf8')).trim();
    const ui = async (id: string) => {
      let found: Locator | undefined;
      await expect
        .poll(
          async () => {
            for (const page of context.pages()) {
              if (!page.url().startsWith(`chrome-extension://${extensionId}/`)) continue;
              const candidate = page.getByTestId(id);
              if ((await candidate.count()) === 1 && (await candidate.isVisible())) {
                found = candidate;
                return true;
              }
            }
            return false;
          },
          { timeout: 20_000 }
        )
        .toBe(true);
      return found!;
    };
    const click = async (id: string) => (await ui(id)).click();
    const fill = async (id: string, value: string) => (await ui(id)).fill(value);
    await click('onboarding-create-wallet');
    await click('onboarding-create-with-srp-button');
    await fill('create-password-new-input', password);
    await fill('create-password-confirm-input', password);
    await click('create-password-terms');
    await click('create-password-submit');
    await click('passkey-maybe-later-button');
    await click('recovery-phrase-remind-later');
    const metrics = await ui('metametrics-checkbox');
    if ((await metrics.getAttribute('data-checked')) === 'true') await metrics.click();
    await click('metametrics-i-agree');
    await click('onboarding-complete-done');
    for (const chain of [125, 126]) {
      await home.goto(`chrome-extension://${extensionId}/home.html#/networks?view=add`);
      await fill('network-form-network-name', `Paxeer isolated ${chain}`);
      await fill('network-form-chain-id', String(chain));
      await home.locator('#nativeCurrency').fill('PAX');
      await click('test-add-rpc-drop-down');
      await home.getByRole('button', { name: 'Add RPC URL', exact: true }).click();
      await fill('rpc-url-input-test', manifest[`rpc${chain}`]);
      await fill('rpc-name-input-test', `Paxeer isolated ${chain}`);
      await click('page-container-footer-next');
      await click('page-container-footer-next');
    }
    const switchChain = async (chain: number) => {
      await home.goto(`chrome-extension://${extensionId}/home.html#`);
      await click('dapp-connection-control-bar__network-button');
      await click(`Paxeer isolated ${chain}`);
    };
    await switchChain(125);
    const app = await context.newPage();
    await app.goto(manifest.app_url);
    await expect(app.getByTestId('embedded-config')).toHaveText('unavailable');
    await expect(app.getByTestId('bootstrap-config-mode')).toHaveText('default');
    const connect = async (approve: boolean) => {
      await app.bringToFront();
      await app
        .locator('[data-testid="injected-connect"][data-provider-rdns="io.metamask"]')
        .click();
      if (approve) {
        await expect
          .poll(async () => {
            if ((await app.getByTestId('wallet-status').textContent()) === 'ready') return 'ready';
            for (const page of context.pages()) {
              if (
                page.url().startsWith(`chrome-extension://${extensionId}/`) &&
                (await page.getByTestId('confirm-btn').isVisible())
              ) {
                await page.getByTestId('confirm-btn').click();
                return 'approved';
              }
            }
            return '';
          })
          .not.toBe('');
      } else await click('cancel-btn');
    };
    await connect(false);
    await expect(app.getByTestId('wallet-status')).toHaveText('signed-out');
    await expect(app.getByTestId('wallet-address')).toHaveText('');
    cases.rejected_connection = true;
    await connect(true);
    await expect(app.getByTestId('wallet-status')).toHaveText('ready');
    const account = (await app.getByTestId('wallet-address').textContent())!.trim();
    expect(account).toMatch(/^0x[0-9a-fA-F]{40}$/);
    await expect(app.getByTestId('wallet-chain')).toHaveText('125');
    cases.missing_embedded_config = { account, chain: 125 };
    for (const chain of [125, 126]) {
      const accounts = await rpcAt(manifest[`rpc${chain}`], 'eth_accounts');
      const hash = await rpcAt(manifest[`rpc${chain}`], 'eth_sendTransaction', [
        {
          from: accounts[0],
          to: account,
          value: '0xde0b6b3a7640000',
          chainId: `0x${chain.toString(16)}`,
        },
      ]);
      await expect
        .poll(
          async () =>
            (await rpcAt(manifest[`rpc${chain}`], 'eth_getTransactionReceipt', [hash]))?.status
        )
        .toBe('0x1');
    }
    await app.getByTestId('wallet-refresh').click();
    await app.getByTestId('wallet-read-chain').click();
    await expect(app.getByTestId('wallet-chain-status')).toHaveText('observed');
    const open = async () => {
      await app.getByTestId('wallet-clear-transfer').click();
      await app.getByTestId('send-recipient').fill(manifest.recipient);
      await app.getByTestId('send-amount').fill('0.0001');
      await app.getByRole('button', { name: 'Send native PAX', exact: true }).click();
      await expect(
        app.getByRole('dialog', { name: 'Confirm send transaction', exact: true })
      ).toBeVisible();
    };
    await open();
    await app.getByRole('button', { name: 'Confirm send', exact: true }).click();
    await click('confirm-footer-button');
    await expect(app.getByTestId('transaction-hash')).toHaveText(/^0x[0-9a-fA-F]{64}$/);
    const hash = (await app.getByTestId('transaction-hash').textContent())!.trim();
    await expect
      .poll(async () => (await rpcAt(manifest.rpc125, 'eth_getTransactionReceipt', [hash]))?.status)
      .toBe('0x1');
    const transaction = await rpcAt(manifest.rpc125, 'eth_getTransactionByHash', [hash]);
    expect(BigInt(transaction.chainId)).toBe(125n);
    expect(transaction.from.toLowerCase()).toBe(account.toLowerCase());
    expect(transaction.to.toLowerCase()).toBe(manifest.recipient.toLowerCase());
    expect(BigInt(transaction.value)).toBe(100000000000000n);
    expect(await rpcAt(manifest.rpc126, 'eth_getTransactionReceipt', [hash])).toBeNull();
    cases.expected_chain_transaction = { hash, chainId: 125 };
    const nonce126 = await rpcAt(manifest.rpc126, 'eth_getTransactionCount', [account, 'pending']);
    await open();
    await switchChain(126);
    await expect(app.getByTestId('wallet-status')).toHaveText('signed-out');
    await expect(app.getByTestId('wallet-address')).toHaveText('');
    await expect(app.getByTestId('wallet-chain')).toHaveText('');
    await expect(
      app.getByRole('dialog', { name: 'Confirm send transaction', exact: true })
    ).toHaveCount(0);
    expect(await rpcAt(manifest.rpc126, 'eth_getTransactionCount', [account, 'pending'])).toBe(
      nonce126
    );
    cases.mid_confirmation_switch = { chainId: 126, nonceUnchanged: true };
    const sdkRefusals = await app.evaluate(async () => {
      const detail = await new Promise<{ provider: any; info: any }>((resolve, reject) => {
        const timer = window.setTimeout(() => {
          window.removeEventListener('eip6963:announceProvider', listener);
          reject(new Error('Actual provider discovery timed out'));
        }, 5000);
        const listener = (event: Event) => {
          const value = (event as CustomEvent).detail;
          if (value?.info?.rdns !== 'io.metamask') return;
          window.clearTimeout(timer);
          window.removeEventListener('eip6963:announceProvider', listener);
          resolve(value);
        };
        window.addEventListener('eip6963:announceProvider', listener);
        window.dispatchEvent(new Event('eip6963:requestProvider'));
      });
      const sdkPath = '/sdk.js';
      const { WalletInterface } = await import(sdkPath);
      const actual = new WalletInterface(detail.provider, detail.info);
      const results = [];
      try {
        for (const action of [
          () => actual.signMessage('isolated chain admission'),
          () =>
            actual.signTypedData({
              domain: { chainId: 125 },
              primaryType: 'Admission',
              types: { Admission: [{ name: 'chain', type: 'uint256' }] },
              message: { chain: 125 },
            }),
          () =>
            actual.sendTransaction({
              to: '0x00000000000000000000000000000000000000b1',
              value: 1n,
              chainId: 125,
            }),
        ]) {
          try {
            await action();
            throw new Error('A wrong-chain SDK request was admitted');
          } catch (error) {
            if ((error as { code?: number }).code !== 4901) throw error;
            results.push(4901);
          }
        }
      } finally {
        actual.dispose();
      }
      return results;
    });
    expect(sdkRefusals).toEqual([4901, 4901, 4901]);
    expect(await rpcAt(manifest.rpc126, 'eth_getTransactionCount', [account, 'pending'])).toBe(
      nonce126
    );
    cases.sdk_wrong_chain_signatures = sdkRefusals;
    await app.reload();
    await expect(app.getByTestId('wallet-status')).toHaveText('signed-out');
    await expect(app.getByTestId('wallet-address')).toHaveText('');
    cases.wrong_chain_reload = true;
    await switchChain(125);
    await connect(true);
    await expect(app.getByTestId('wallet-status')).toHaveText('ready');
    await expect(app.getByTestId('wallet-chain')).toHaveText('125');
    await expect(app.getByTestId('wallet-address')).toHaveText(account);
    await app.reload();
    await expect(app.getByTestId('wallet-status')).toHaveText('ready');
    await expect(app.getByTestId('wallet-chain')).toHaveText('125');
    cases.reconnect_reload = { account, chainId: 125 };
    await open();
    await home.goto(`chrome-extension://${extensionId}/home.html#`);
    await click('account-options-menu-button');
    await click('global-menu-lock');
    await expect(app.getByTestId('wallet-status')).toHaveText('signed-out');
    await expect(
      app.getByRole('dialog', { name: 'Confirm send transaction', exact: true })
    ).toHaveCount(0);
    cases.accounts_changed_lock = true;
    await fill('unlock-password', password);
    await click('unlock-submit');
    await connect(true);
    await expect(app.getByTestId('wallet-status')).toHaveText('ready');
    await open();
    await app.getByTestId('wallet-sign-out').click();
    await expect(app.getByTestId('wallet-status')).toHaveText('signed-out');
    await expect(
      app.getByRole('dialog', { name: 'Confirm send transaction', exact: true })
    ).toHaveCount(0);
    cases.sign_out = true;
    await connect(true);
    await expect(app.getByTestId('wallet-status')).toHaveText('ready');
    await open();
    await home.goto(`chrome-extension://${extensionId}/home.html#`);
    await click('account-options-menu-button');
    await click('global-menu-connected-sites');
    await click('disconnect-all-button');
    await click('disconnect-all-sites-confirm');
    await expect(app.getByTestId('wallet-status')).toHaveText('signed-out');
    await expect(app.getByTestId('wallet-address')).toHaveText('');
    await expect(
      app.getByRole('dialog', { name: 'Confirm send transaction', exact: true })
    ).toHaveCount(0);
    expect(await rpcAt(manifest.rpc126, 'eth_getTransactionCount', [account, 'pending'])).toBe(
      nonce126
    );
    cases.revoked_connection = true;
    await fs.writeFile(
      path.join(manifest.output_dir, 'chain-result.json'),
      JSON.stringify({
        schema: 'paxeer-x.wallet-injected-chain-result.v1',
        source_hashes: manifest.source_hashes,
        cases,
        completed_cases: Object.keys(cases).length,
        skipped_cases: 0,
        exit_code: 0,
      }),
      { mode: 0o600 }
    );
  } finally {
    await context.close();
  }
});

async function send_reaches_committed_only_with_a_matching_explorer_answer(
  { page, baseURL }: { page: Page; baseURL: string | undefined },
  testInfo: TestInfo
): Promise<void> {
  const sequencer = (method: string, params: readonly unknown[]) =>
    rpcAt(SEQUENCER_RPC, method, params);
  let matchReceiptBlock = false;
  const answers: {
    hash: string;
    rung: 'committed';
    block: number;
    phase: 'mismatched' | 'matching';
  }[] = [];
  await page.route('**/api/v2/transactions/*/status', async (route) => {
    const requestedHash = new URL(route.request().url()).pathname.match(
      /transactions\/(0x[0-9a-fA-F]{64})\/status$/
    )?.[1];
    if (!requestedHash) throw new Error('Malformed explorer request');
    const receipt = await readSequencerReceipt(sequencer, requestedHash);
    if (receipt?.rung !== 'committed') {
      await route.fulfill({
        json: {
          rung: 'submitted',
          hash: requestedHash,
          block_number: null,
          committed_height: null,
        },
      });
      return;
    }
    expect(receipt).toMatchObject({ txHash: requestedHash, rung: 'committed', status: '0x1' });
    if (receipt?.blockNumber === undefined || receipt.blockHash === undefined)
      throw new Error('Explorer answers require a committed sequencer receipt block');
    const phase = matchReceiptBlock ? 'matching' : 'mismatched';
    const blockNumber = Number(BigInt(receipt.blockNumber)) + (matchReceiptBlock ? 0 : 1);
    const status = {
      rung: 'committed' as const,
      hash: requestedHash,
      block_number: blockNumber,
      committed_height: blockNumber,
    };
    await route.fulfill({ json: status });
    answers.push({ hash: requestedHash, rung: status.rung, block: status.block_number, phase });
  });

  await send_shows_receipted_from_the_sequencer_receipt_and_keeps_it_on_reload(
    { page, baseURL },
    testInfo
  );
  const submitted = await page.evaluate((sender) => {
    const raw = localStorage.getItem('paxport:v1:submitted-transfers');
    if (raw === null) throw new Error('The sent transfer identity was not saved');
    const stored = JSON.parse(raw) as {
      value: Record<string, { hash: string; chainId: number; sender: string }>;
    };
    const transfer = stored.value[`125:${sender.toLowerCase()}`];
    if (transfer === undefined) throw new Error('The sender has no saved transfer identity');
    return transfer;
  }, ANVIL_SENDER);
  expect(submitted.chainId).toBe(125);
  expect(submitted.sender.toLowerCase()).toBe(ANVIL_SENDER.toLowerCase());
  const { hash } = submitted;
  expect(hash).toMatch(/^0x[0-9a-fA-F]{64}$/);
  await expect
    .poll(async () => (await readSequencerReceipt(sequencer, hash))?.rung, {
      intervals: [50],
      timeout: 60_000,
    })
    .toBe('committed');
  const receipt = await readSequencerReceipt(sequencer, hash);
  if (receipt?.blockNumber === undefined || receipt.blockHash === undefined)
    throw new Error('The sent transfer requires a committed sequencer receipt block');
  expect(receipt).toMatchObject({ txHash: hash, rung: 'committed', status: '0x1' });
  const blockNumber = Number(BigInt(receipt.blockNumber));
  const blockHash = receipt.blockHash.toLowerCase();

  const savedObservation = () =>
    page.evaluate((key) => {
      const raw = localStorage.getItem('paxport:v1:transfer-observations');
      if (raw === null) throw new Error('The transfer observation was not saved');
      const stored = JSON.parse(raw) as { value: Record<string, TransferObservation> };
      return stored.value[key];
    }, `125:${hash.toLowerCase()}`);
  const committed = page.locator('li[data-rung="committed"]');
  await expect.poll(savedObservation, { intervals: [50], timeout: 60_000 }).toMatchObject({
    truth: 'included',
    blockNumber,
    blockHash,
    sequencer: receipt,
    steps: [{ rung: 'receipted', source: 'receipt', state: `receipted:${blockHash}` }],
    warning: 'explorer status does not identify the receipt block',
  });
  await expect
    .poll(() => answers, { intervals: [50], timeout: 60_000 })
    .toContainEqual({
      hash,
      rung: 'committed',
      block: blockNumber + 1,
      phase: 'mismatched',
    });
  expect(answers.filter((answer) => answer.phase === 'matching')).toHaveLength(0);
  await expect(title(page)).toHaveText('Receipted');
  await expect(committed).toHaveAttribute('data-reached', 'false');
  await expect(committed).not.toHaveAttribute('data-source');
  await expect(committed).not.toHaveAttribute('data-evidence');
  await expect(page.getByText('Transfer Completed')).toHaveCount(0);
  await expect(
    page
      .getByRole('alert')
      .filter({ hasText: 'explorer status does not identify the receipt block' })
  ).toBeVisible();

  matchReceiptBlock = true;
  await page.evaluate(() => window.dispatchEvent(new Event('online')));
  const steps = [
    { rung: 'committed', source: 'receipt', state: `committed:${blockHash}` },
    { rung: 'committed', source: 'explorer', state: `committed:${blockHash}` },
  ];
  await expect.poll(savedObservation, { intervals: [50], timeout: 60_000 }).toMatchObject({
    identity: { hash, chainId: 125 },
    truth: 'included',
    blockNumber,
    blockHash,
    sequencer: receipt,
    steps,
    lastVerified: { blockNumber, blockHash, sequencer: receipt, steps },
  });
  await expect
    .poll(() => answers, { intervals: [50], timeout: 60_000 })
    .toContainEqual({
      hash,
      rung: 'committed',
      block: blockNumber,
      phase: 'matching',
    });
  await expect(committed.getByText('Committed', { exact: true })).toBeVisible();
  await expect(committed).toHaveAttribute('data-reached', 'true');
  await expect(committed).toHaveAttribute('data-source', 'explorer');
  await expect(committed).toHaveAttribute('data-evidence', `committed:${blockHash}`);
  await expect(page.getByText('Transfer Completed')).toBeVisible();
  await expect(page.getByRole('alert').filter({ hasText: /\S/ })).toHaveCount(0);
  await page.reload();
  await expect(committed).toHaveAttribute('data-reached', 'true');
  await expect(committed).toHaveAttribute('data-source', 'explorer');
  await expect(committed).toHaveAttribute('data-evidence', `committed:${blockHash}`);
  await expect(page.getByText('Transfer Completed')).toBeVisible();
}

async function send_shows_receipted_from_the_sequencer_receipt_and_keeps_it_on_reload(
  { page, baseURL }: { page: Page; baseURL: string | undefined },
  testInfo: TestInfo
): Promise<void> {
  const key = process.env.WALLET_TRANSFER_TRUTH_SEQUENCER_KEY;
  if (!SEQUENCER_RPC || !key || !/^0x[0-9a-fA-F]{64}$/.test(key))
    throw new Error('The local sequencer RPC and its funded signing key are required');
  const account = privateKeyToAccount(key as Hex);
  expect(account.address.toLowerCase()).toBe(ANVIL_SENDER.toLowerCase());
  const chainId = Number(BigInt(await rpcAt(SEQUENCER_RPC, 'eth_chainId')));
  expect(chainId).toBe(125);
  const wallet = createWalletClient({ account, transport: http(SEQUENCER_RPC) });
  const announcedProviders: { uuid: string; name: string; rdns: string }[] = [];
  const browserErrors: string[] = [];
  page.on('pageerror', (error) => browserErrors.push(error.message));
  page.on('console', (message) => {
    if (message.type() === 'error' || message.type() === 'warning')
      browserErrors.push(`${message.type()}: ${message.text()}`);
  });
  await page.exposeBinding(
    'recordSequencerProviderAnnouncement',
    (_source, info: { uuid: string; name: string; rdns: string }) => {
      announcedProviders.push(info);
    }
  );
  await page.exposeBinding(
    'sequencerWalletRequest',
    async (_source, request: { method: string; params?: readonly unknown[] }) => {
      if (request.method === 'eth_accounts' || request.method === 'eth_requestAccounts')
        return wallet.getAddresses();
      return rpcAt(SEQUENCER_RPC, request.method, request.params);
    }
  );
  await page.addInitScript(() => {
    if (sessionStorage.getItem('sequencer-wallet-session-cleared') === null) {
      localStorage.removeItem('paxport:v1:custody-choice');
      localStorage.removeItem('paxeer:wallet-kind');
      for (const key of Object.keys(localStorage)) {
        if (/^sb-.*-auth-token$/.test(key)) localStorage.removeItem(key);
      }
      sessionStorage.setItem('sequencer-wallet-session-cleared', 'true');
    }
    const events = new EventTarget();
    const provider = {
      request: (request: { method: string; params?: readonly unknown[] }) =>
        (
          window as unknown as {
            sequencerWalletRequest: (request: {
              method: string;
              params?: readonly unknown[];
            }) => Promise<unknown>;
          }
        ).sequencerWalletRequest(request),
      on: (event: string, listener: EventListener) => events.addEventListener(event, listener),
      removeListener: (event: string, listener: EventListener) =>
        events.removeEventListener(event, listener),
    };
    const detail = {
      info: {
        uuid: crypto.randomUUID(),
        name: 'Local sequencer signer',
        icon: 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg"/>',
        rdns: 'local.sequencer.wallet',
      },
      provider,
    };
    const announce = () => {
      void (
        window as unknown as {
          recordSequencerProviderAnnouncement: (info: typeof detail.info) => Promise<void>;
        }
      ).recordSequencerProviderAnnouncement(detail.info);
      window.dispatchEvent(new CustomEvent('eip6963:announceProvider', { detail }));
    };
    window.addEventListener('eip6963:requestProvider', announce);
    announce();
  });
  observationRpc = SEQUENCER_RPC;
  await routeObservations(page);
  if (!baseURL) throw new Error('The wallet app base URL is required');
  const document = await fetch(new URL('./', baseURL));
  const documentBody = await document.text();
  console.log('Wallet app response before navigation', {
    url: document.url,
    status: document.status,
    headers: Object.fromEntries(document.headers),
    bodyHead: documentBody.slice(0, 300),
  });
  console.log(
    'Wallet app dev server stdout',
    await readFile(join(tmpdir(), 'paxeer-wallet-app-webserver.log'), 'utf8')
  );
  expect(document.status).toBe(200);
  expect(document.headers.get('content-type')).toContain('text/html');
  expect(documentBody).toContain('Paxeer Wallet');
  const navigation = await page.goto('./');
  await page.evaluate(() => window.dispatchEvent(new Event('eip6963:requestProvider')));
  try {
    await expect(page.getByLabel('Loading wallet', { exact: true })).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Paxeer Wallet', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: /Sign in with email or social/ })).toBeVisible();
    await expect(page.getByRole('button', { name: /Local sequencer signer/ })).toBeVisible();
  } catch (error) {
    console.error('Sequencer wallet onboarding diagnostics', {
      url: page.url(),
      status: navigation?.status(),
      headers: navigation?.headers(),
      contentType: navigation?.headers()['content-type'],
      contentLength: (await page.content()).length,
      nextjsPortal: await page.locator('nextjs-portal').allTextContents(),
      serverStdout: await readFile(join(tmpdir(), 'paxeer-wallet-app-webserver.log'), 'utf8'),
      responseHead: (await navigation?.text())?.slice(0, 500),
      body: await page.locator('body').innerText(),
      announcedProviders,
      browserErrors,
    });
    throw error;
  }
  await page.getByRole('button', { name: /Local sequencer signer/ }).click();
  await expect(bottomTab(page, 'Settings')).toBeVisible();

  let gasPrice: bigint | undefined;
  await expect
    .poll(
      async () => {
        try {
          gasPrice = BigInt(await rpcAt(SEQUENCER_RPC, 'eth_gasPrice'));
          return true;
        } catch (error) {
          if (
            error instanceof Error &&
            (error.message.includes('pending base fee unavailable: sequencer: no block is open') ||
              error.message.includes('pending block unavailable: sequencer: no block is open'))
          )
            return false;
          throw error;
        }
      },
      { intervals: [50], timeout: 60_000 }
    )
    .toBe(true);
  if (gasPrice === undefined) throw new Error('The sequencer did not answer eth_gasPrice');
  const signed = await account.signTransaction({
    chainId,
    type: 'legacy',
    nonce: Number(
      BigInt(await rpcAt(SEQUENCER_RPC, 'eth_getTransactionCount', [account.address, 'latest']))
    ),
    to: RECIPIENT,
    value: 1n,
    gas: 100_000n,
    gasPrice,
  });
  let hash = '';
  await expect
    .poll(
      async () => {
        try {
          hash = await rpcAt(SEQUENCER_RPC, 'eth_sendRawTransaction', [signed]);
          return true;
        } catch (error) {
          if (
            error instanceof Error &&
            error.message.includes('pending block unavailable: sequencer: no block is open')
          )
            return false;
          throw error;
        }
      },
      { intervals: [50], timeout: 60_000 }
    )
    .toBe(true);
  expect(hash).toMatch(/^0x[0-9a-fA-F]{64}$/);
  await observe(page, hash);
  try {
    await expect
      .poll(
        async () =>
          ['pending', 'submitted', 'included'].includes(
            (await title(page).getAttribute('data-truth')) ?? ''
          ),
        { intervals: [50], timeout: 60_000 }
      )
      .toBe(true);
  } catch (error) {
    await testInfo.attach('send-page.html', {
      body: await page.content(),
      contentType: 'text/html',
    });
    await testInfo.attach('wallet-server.log', {
      body: await readFile(join(tmpdir(), 'paxeer-wallet-app-webserver.log')),
      contentType: 'text/plain',
    });
    console.error('Sequencer transfer render diagnostics', {
      url: page.url(),
      body: await page.locator('body').innerText(),
      nextjsPortal: await page.locator('nextjs-portal').allTextContents(),
      browserErrors,
    });
    throw error;
  }
  const sequencer = (method: string, params: readonly unknown[]) =>
    rpcAt(SEQUENCER_RPC, method, params);
  await expect
    .poll(async () => (await readSequencerReceipt(sequencer, hash))?.blockHash, {
      intervals: [50],
      timeout: 60_000,
    })
    .toBeDefined();
  const receipt = await readSequencerReceipt(sequencer, hash);
  expect(receipt).toMatchObject({ txHash: hash, status: '0x1' });
  expect(receipt?.rung).toMatch(/^(receipted|committed)$/);
  expect(receipt?.blockHash).toMatch(/^0x[0-9a-fA-F]{64}$/);
  if (receipt === null || receipt.blockHash === undefined)
    throw new Error('Receipted evidence requires a block hash');
  const step = {
    rung: 'receipted',
    source: 'receipt',
    state: `receipted:${receipt.blockHash.toLowerCase()}`,
  };

  const savedObservation = () =>
    page.evaluate((key) => {
      const raw = window.localStorage.getItem('paxport:v1:transfer-observations');
      if (raw === null) throw new Error('The transfer observation was not saved for reload');
      const stored = JSON.parse(raw) as { value: Record<string, TransferObservation> };
      return stored.value[key];
    }, `125:${hash.toLowerCase()}`);

  await expect(title(page)).toHaveAttribute('data-truth', 'included');
  await expect(title(page)).toHaveText('Receipted');
  const receipted = page.locator('li[data-rung="receipted"]');
  await expect(receipted.getByText('Receipted', { exact: true })).toBeVisible();
  await expect(receipted).toHaveAttribute('data-reached', 'true');
  await expect(receipted).toHaveAttribute('data-source', 'receipt');
  await expect(receipted).toHaveAttribute('data-evidence', step.state);
  await expect(page.locator('li[data-rung="committed"]')).toHaveAttribute('data-reached', 'false');
  await expect(page.getByText('Transfer Completed')).toHaveCount(0);
  await expect.poll(savedObservation, { intervals: [50], timeout: 60_000 }).toMatchObject({
    identity: { hash, chainId: 125 },
    truth: 'included',
    steps: [step],
    sequencer: receipt,
    lastVerified: { steps: [step], sequencer: receipt },
  });

  await page.reload();
  await expect(title(page)).toHaveAttribute('data-truth', 'included');
  await expect(title(page)).toHaveText('Receipted');
  await expect(receipted.getByText('Receipted', { exact: true })).toBeVisible();
  await expect(receipted).toHaveAttribute('data-reached', 'true');
  await expect(receipted).toHaveAttribute('data-source', 'receipt');
  await expect(receipted).toHaveAttribute('data-evidence', step.state);
  await expect(page.locator('li[data-rung="committed"]')).toHaveAttribute('data-reached', 'false');
  await expect(page.getByText('Transfer Completed')).toHaveCount(0);
  await expect.poll(savedObservation, { intervals: [50], timeout: 60_000 }).toMatchObject({
    identity: { hash, chainId: 125 },
    truth: 'included',
    steps: [step],
    sequencer: receipt,
    lastVerified: { steps: [step], sequencer: receipt },
  });
}
