/**
 * Proves: bridge-card.tsx BridgeCard/recordBridgeSubmission -> transfers.ts localStorage + transfers-panel.tsx transferListOptions/transferLiveOptions -> page.tsx Home/TransfersPanel -> deposit-status.tsx DepositStatus/TransferSteps.
 * Also drives transfer-actions.ts buildVerifiedClaim/buildVerifiedRelease/readTransferLive,
 * TransferLive and deposit-status.tsx TransferStep through their real exported modules.
 * Repairs: settlement builders accepted non-ready attestations; readTransferLive accepted
 * unknown chains; the status consumer silently discarded malformed progress steps and
 * trusted malformed live state. No transport, hook, component or storage is replaced.
 * The unconfigured deployment's refusal paths are asserted, not replaced with a chain.
 * No workspace dist entries are imported.
 */
// @vitest-environment jsdom

import { componentReleaseConsumesPersistedSubmissionAndRefusesSignedFieldChanges } from './repair-22-33-266.proofs';

import { QueryClient, QueryClientProvider, QueryObserver } from '@tanstack/react-query';
import { createElement, type FunctionComponentElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { encodeFunctionData, keccak256, zeroAddress } from 'viem';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import { expect, test } from 'vitest';
import { WagmiProvider } from 'wagmi';

import Home from '@/app/page';
import { BridgeCard, recordBridgeSubmission } from '@/components/bridge/bridge-card';
import {
  decodeTransferLive,
  readTransferLive,
  type TransferLive,
} from '@/components/bridge/transfer-actions';
import {
  transferListOptions,
  transferLiveOptions,
  TransfersPanel,
} from '@/components/bridge/transfers-panel';
import { ThemeProvider } from '@/components/theme-provider';
import { DepositStatus, TransferSteps, type TransferStatusItem, type TransferStep } from '@/components/trading/deposit-status';
import { TooltipProvider } from '@/components/ui/tooltip';
import { bridgeChainByKey, paxeer } from '@/lib/bridge/chains';
import { phaseOf, stepOf } from '@/lib/bridge/phase';
import { planDeposit } from '@/lib/bridge/submission';
import { listTransfers, updateTransfer } from '@/lib/bridge/transfers';
import { wagmiConfig } from '@/lib/bridge/wagmi';

const wallet = privateKeyToAccount(generatePrivateKey());
const vault = privateKeyToAccount(generatePrivateKey()).address;

function requiredChain(key: string) {
  const chain = bridgeChainByKey(key);
  if (!chain) throw new Error(`Missing production route ${ key }.`);
  return chain;
}

async function signedSubmission() {
  const foreign = requiredChain('base');
  const asset = foreign.assets[0];
  const amount = 1250000000000000000n;
  const [ request ] = planDeposit({ asset, amount, allowance: 0n, vault, recipient: wallet.address });
  const data = request.functionName === 'approve' ? encodeFunctionData(request) : encodeFunctionData(request);
  const serialized = await wallet.signTransaction({
    chainId: Number(foreign.chainId),
    type: 'eip1559',
    nonce: 0,
    gas: 200000n,
    maxFeePerGas: 2n,
    maxPriorityFeePerGas: 1n,
    to: vault,
    data,
    value: 'value' in request ? request.value : 0n,
  });
  return { hash: keccak256(serialized), foreign, asset, account: wallet.address, amount, requiredConfirmations: foreign.finalityDepth };
}

function renderConsumer<Props extends object>(client: QueryClient, component: FunctionComponentElement<Props>) {
  return renderToStaticMarkup(createElement(ThemeProvider, null,
    createElement(WagmiProvider, { config: wagmiConfig, reconnectOnMount: false },
      createElement(QueryClientProvider, { client },
        createElement(TooltipProvider, null, component),
      ),
    ),
  ));
}

async function components_publish_cross_storage_and_reopen_the_real_page() {
  const originalState = wagmiConfig.state;
  const originalStorage = window.localStorage.getItem('paxeer-bridge:transfers:v1');
  const clients: QueryClient[] = [];
  try {
    window.localStorage.removeItem('paxeer-bridge:transfers:v1');
    const input = await signedSubmission();
    let refreshKey = 0;
    const transfer = recordBridgeSubmission({ direction: 'in', input: { ...input, vault, recipient: wallet.address } }, () => {
      // This is the page callback's observable contract: storage is already visible.
      expect(listTransfers(wallet.address)).toHaveLength(1);
      refreshKey += 1;
    });
    expect(refreshKey).toBe(1);
    expect(transfer.sourceTxHash).toBe(input.hash);
    expect(transfer.amount).toBe(input.amount.toString());

    // Chain-derived identifiers are deliberately absent. Solana has no EVM receipt
    // reader (lib/bridge/chains.ts buildBridgeChains); its current expected behavior
    // in readTransferLive is zero confirmations, never a fabricated credit.
    const solana = requiredChain('solana');
    const solanaTransfer = recordBridgeSubmission({
      direction: 'in',
      input: { ...input, foreign: solana, asset: solana.assets[0], amount: 1250000000n, vault, recipient: wallet.address },
    }, () => {
      refreshKey += 1;
    });
    expect(refreshKey).toBe(2);
    expect(solanaTransfer.id).toBe(transfer.id);

    const connector = wagmiConfig.connectors[0];
    if (!connector) throw new Error('The production injected connector is missing.');
    wagmiConfig.setState({
      ...originalState,
      chainId: paxeer.id,
      current: connector.uid,
      connections: new Map([ [ connector.uid, { accounts: [ wallet.address ], chainId: paxeer.id, connector } ] ]),
      status: 'connected',
    });

    for (const expectedAmount of [ solanaTransfer.amount, '2500000000' ]) {
      // A fresh client and a new render discard the previous consumer's memory.
      const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
      clients.push(client);
      // Home starts at refreshKey zero; the explicit panel below receives the
      // submission callback's current key. Both execute the production query.
      const records = await client.fetchQuery(transferListOptions(wallet.address, 0, 0));
      await client.fetchQuery(transferListOptions(wallet.address, refreshKey, 0));
      expect(records).toHaveLength(1);
      expect(records[0].amount).toBe(expectedAmount);
      const observer = new QueryObserver(client, transferLiveOptions(records[0]));
      const observed = await observer.refetch();
      expect(observed.isSuccess).toBe(true);
      const live: TransferLive | undefined = observed.data;
      expect(live).toEqual({
        confirmations: 0,
        requiredConfirmations: input.requiredConfirmations,
        credited: false,
        released: false,
        attestationReady: false,
        sourceFailed: false,
      });
      expect(stepOf(phaseOf(records[0], live, false))).toBe('confirming');

      const html = renderConsumer(client, createElement(Home));
      expect(html).toContain('Move assets to Paxeer');
      expect(html).toContain('Your transfers');
      expect(html).toContain('You have a pending transaction');
      expect(html).toContain('Solana');
      expect(html).toContain('Processing');
      expect(html).toContain(expectedAmount === solanaTransfer.amount ? '1.25' : '2.5');
      expect(html).not.toContain('You have a successful transaction');
      expect(renderConsumer(client, createElement(TransfersPanel, { refreshKey }))).toContain('You have a pending transaction');
      expect(renderConsumer(client, createElement(BridgeCard, { onTransferSubmitted: () => {
        refreshKey += 1;
      } })))
        .toContain('Deposit unavailable');
      observer.destroy();
      client.clear();
      // The next consumer must observe this producer update, not its old query data.
      updateTransfer(solanaTransfer.id, { amount: '2500000000' });
    }
    const otherWalletClient = new QueryClient({ defaultOptions: { queries: { gcTime: 0 } } });
    clients.push(otherWalletClient);
    expect(await otherWalletClient.fetchQuery(transferListOptions(zeroAddress, refreshKey, 0))).toEqual([]);
  } finally {
    for (const client of clients) client.clear();
    wagmiConfig.setState(originalState);
    if (originalStorage === null) window.localStorage.removeItem('paxeer-bridge:transfers:v1');
    else window.localStorage.setItem('paxeer-bridge:transfers:v1', originalStorage);
  }
}

async function components_decode_live_results_and_progress_without_a_cached_shortcut() {
  const input = await signedSubmission();
  const solana = requiredChain('solana');
  const originalStorage = window.localStorage.getItem('paxeer-bridge:transfers:v1');
  try {
    const transfer = recordBridgeSubmission({
      direction: 'in', input: { ...input, foreign: solana, vault, recipient: wallet.address },
    }, () => {});
    for (const requiredConfirmations of [ 12, 24 ]) {
      const live: TransferLive = await readTransferLive({ ...transfer, requiredConfirmations });
      expect(decodeTransferLive(live).requiredConfirmations).toBe(requiredConfirmations);
      const step: TransferStep = stepOf(phaseOf(transfer, live, false));
      const item: TransferStatusItem = {
        id: transfer.id, token: transfer.symbol, amount: 1.25, step,
        confirmations: live.confirmations, requiredConfirmations: live.requiredConfirmations,
      };
      const html = renderToStaticMarkup(createElement(DepositStatus, { items: [ item ], open: true }));
      expect(html).toContain(`Confirming 0/${ requiredConfirmations }`);
      expect(html).toContain('aria-current="step"');
      expect(html).not.toContain('You have a successful transaction');

      // Damage a real producer result at the exact exported consumer decoder.
      for (const malformed of [
        { ...live, confirmations: -1 },
        { ...live, requiredConfirmations: '24' },
        { ...live, credited: 'true' },
        { ...live, released: null },
        { ...live, attestationReady: 1 },
        { ...live, sourceFailed: undefined },
      ]) expect(() => decodeTransferLive(malformed)).toThrow('Invalid live transfer state.');
      const malformedItem: TransferStatusItem = JSON.parse(JSON.stringify({ ...item, step: 'unknown' }));
      expect(() => renderToStaticMarkup(createElement(DepositStatus, { items: [ malformedItem ] })))
        .toThrow('Invalid transfer progress step.');
      expect(() => renderToStaticMarkup(createElement(TransferSteps, { item: malformedItem })))
        .toThrow('Invalid transfer progress step.');
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
    const observer = new QueryObserver(client, transferLiveOptions({ ...transfer, requiredConfirmations: -1 }));
    try {
      // The real producer emits the damaged input, and the panel's real query
      // selection refuses it. There is no injected result or replaced queryFn.
      const observed = await observer.refetch();
      expect(observed.isError).toBe(true);
      expect(observed.error?.message).toBe('Invalid live transfer state.');
      expect(observed.data).toBeUndefined();
    } finally {
      observer.destroy();
      client.clear();
    }
    await expect(readTransferLive({ ...transfer, foreignChainKey: 'unknown' })).rejects.toThrow('Unknown or mismatched bridge chain.');
    await expect(readTransferLive({ ...transfer, foreignChainId: '8453' })).rejects.toThrow('Unknown or mismatched bridge chain.');
    for (const step of [ 'submitted', 'confirming', 'attesting', 'ready', 'credited', 'failed' ] as const) {
      const html = renderToStaticMarkup(createElement(DepositStatus, {
        items: [ { id: transfer.id, token: transfer.symbol, amount: 1.25, step } ], open: true,
      }));
      expect(html).toContain(step === 'credited' ? 'You have a successful transaction' : step === 'failed' ? 'A transaction failed' : 'You have a pending transaction');
    }
  } finally {
    if (originalStorage === null) window.localStorage.removeItem('paxeer-bridge:transfers:v1');
    else window.localStorage.setItem('paxeer-bridge:transfers:v1', originalStorage);
  }
}

async function components_verify_real_signatures_and_refuse_unready_or_mismatched_settlement() {
  await componentReleaseConsumesPersistedSubmissionAndRefusesSignedFieldChanges();
}

test(components_publish_cross_storage_and_reopen_the_real_page);
test(components_decode_live_results_and_progress_without_a_cached_shortcut);
test(components_verify_real_signatures_and_refuse_unready_or_mismatched_settlement.name, components_verify_real_signatures_and_refuse_unready_or_mismatched_settlement);
