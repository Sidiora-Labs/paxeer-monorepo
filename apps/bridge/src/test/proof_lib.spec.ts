/**
 * Proves: chains.ts/BRIDGE_CHAINS -> exported lib/bridge TypeScript modules -> page.tsx/Home and bridge consumers.
 * Also drives amount.ts parseAmount, errors.ts describeWriteError,
 * attestation-api.ts fetchDepositAttestation/fetchReleaseAttestation, digests.ts
 * verifyThresholdSignatures, explorer.ts paxeerAddressUrl, transfers.ts listTransfers
 * -> their exported TypeScript modules, real HTTP/JSON and localStorage -> page.tsx
 * Home, bridge-card.tsx bridgeCardAmount/recordBridgeSubmission, how-it-works.tsx
 * HowItWorks, site-header.tsx walletNetworks/notifyWalletError, wagmi.ts wagmiConfig,
 * transfers-panel.tsx query options/history and transfer-actions.ts identity consumers.
 * Repairs: parseAmount accepted invalid asset decimals; verifyThresholdSignatures
 * accepted NaN/negative/fractional thresholds; ledger decoding accepted integers
 * outside contract widths; error lookup admitted inherited properties; HowItWorks
 * rendered a link without an explorer. Wallet network selection now shares its
 * EVM-only projection. All eight producers are observed twice without cached queries.
 * The attestor application harness signs real digests and serves its own bundles
 * over a loopback listener. No environment inputs, operator services, dist imports,
 * mocked transports or replaced modules. The local ledger is reopened before use.
 */
// @vitest-environment jsdom

import { libraryHttpBundleReachesReloadedWithdrawalAndStrictThresholdChecks } from './repair-22-33-266.proofs';

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type FunctionComponentElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { BaseError, ContractFunctionRevertedError, encodeErrorResult, encodeFunctionData, keccak256 } from 'viem';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import { expect, test, vi } from 'vitest';
import { WagmiProvider } from 'wagmi';

import Home from '@/app/page';
import { bridgeCardAmount, recordBridgeSubmission } from '@/components/bridge/bridge-card';
import { HowItWorks } from '@/components/bridge/how-it-works';
import { notifyWalletError, walletNetworks } from '@/components/bridge/site-header';
import { transferHistoryRecords, transferListOptions, TransfersPanel } from '@/components/bridge/transfers-panel';
import { ThemeProvider } from '@/components/theme-provider';
import { TransferHistory } from '@/components/trading/transfer-history';
import { toast } from '@/components/ui/sonner';
import { TooltipProvider } from '@/components/ui/tooltip';
import { parseAmount } from '@/lib/bridge/amount';
import { BRIDGE_CHAINS, bridgeChainByKey, buildBridgeChains, paxeer, walletBridgeChains } from '@/lib/bridge/chains';
import { bridgeConfig, readConfig } from '@/lib/bridge/config';
import { LAYERX_BRIDGE_ADDRESS, layerxBridgeAbi } from '@/lib/bridge/contracts';
import { describeWriteError } from '@/lib/bridge/errors';
import { paxeerAddressUrl } from '@/lib/bridge/explorer';
import { planDeposit } from '@/lib/bridge/submission';
import { listTransfers, updateTransfer } from '@/lib/bridge/transfers';
import { wagmiConfig } from '@/lib/bridge/wagmi';
import { useDepositAttestation, useReleaseAttestation } from '@/lib/bridge/use-bridge-state';

const wallet = privateKeyToAccount(generatePrivateKey());
const attestor = privateKeyToAccount(generatePrivateKey());
const vault = privateKeyToAccount(generatePrivateKey()).address;
const storageKey = 'paxeer-bridge:transfers:v1';

function requiredChain(key: string) {
  const chain = bridgeChainByKey(key);
  if (!chain) throw new Error(`apps/bridge/src/lib/bridge/chains.ts is missing ${ key }.`);
  return chain;
}

function newClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
}

function renderConsumer<Props extends object>(client: QueryClient, component: FunctionComponentElement<Props>) {
  return renderToStaticMarkup(createElement(ThemeProvider, null,
    createElement(WagmiProvider, { config: wagmiConfig, reconnectOnMount: false },
      createElement(QueryClientProvider, { client }, createElement(TooltipProvider, null, component)),
    ),
  ));
}

async function signedSubmission(input = '1.25') {
  const foreign = requiredChain('base');
  const asset = foreign.assets[0];
  const { parsedAmount: amount } = bridgeCardAmount(input, asset.decimals);
  if (amount === undefined) throw new Error('amount.ts refused the valid submission amount.');
  const [ request ] = planDeposit({ asset, amount, allowance: 0n, vault, recipient: wallet.address });
  const serialized = await wallet.signTransaction({
    chainId: Number(foreign.chainId), type: 'eip1559', nonce: 0, gas: 200000n,
    maxFeePerGas: 2n, maxPriorityFeePerGas: 1n, to: vault,
    data: request.functionName === 'approve' ? encodeFunctionData(request) : encodeFunctionData(request), value: 'value' in request ? request.value : 0n,
  });
  return { hash: keccak256(serialized), foreign, asset, account: wallet.address, amount, requiredConfirmations: foreign.finalityDepth };
}

function AttestationHookRefusal() {
  const deposit = useDepositAttestation(undefined, false);
  const release = useReleaseAttestation(undefined, false);
  return createElement('output', null, `${ deposit.fetchStatus }:${ release.fetchStatus }`);
}

function proofLibRegistryExplorerAndErrorsReachRealConsumersTwice() {
  for (let observation = 0; observation < 2; observation++) {
    const client = newClient();
    try {
      expect(buildBridgeChains(bridgeConfig)).toEqual(BRIDGE_CHAINS);
      expect(renderConsumer(client, createElement(AttestationHookRefusal))).toBe('<output>idle:idle</output>');
      const networks = walletNetworks();
      expect(networks.map((network) => network.chainId)).toEqual(wagmiConfig.chains.map((chain) => chain.id));
      expect(networks).toEqual([
        { id: 'paxeer', name: 'Paxeer', chainId: 125 },
        ...BRIDGE_CHAINS.filter((chain) => chain.kind === 'evm').map((chain) => ({
          id: chain.key, name: chain.label, chainId: Number(chain.chainId),
        })),
      ]);
      expect(networks.some((network) => network.id === 'solana')).toBe(false);
      const malformed = buildBridgeChains(readConfig({}));
      const solana = malformed.find((chain) => chain.key === 'solana');
      if (!solana) throw new Error('chains.ts is missing the Solana producer.');
      solana.viemChain = paxeer;
      expect(walletBridgeChains(malformed).some((chain) => chain.key === 'solana')).toBe(false);

      const home = renderConsumer(client, createElement(Home));
      expect(home).toContain(BRIDGE_CHAINS.map((chain) => chain.label).join(', '));
      const configured = BRIDGE_CHAINS.filter((chain) => chain.state === 'configured').length;
      expect(home).toContain(configured ? `${ configured } of ${ BRIDGE_CHAINS.length } routes live` : 'Routes open as vaults deploy');
      const base = 'http://127.0.0.1/explorer';
      const url = paxeerAddressUrl(LAYERX_BRIDGE_ADDRESS, base);
      expect(url).toBe(`${ base }/address/${ LAYERX_BRIDGE_ADDRESS }`);
      const how = renderConsumer(client, createElement(HowItWorks, { explorerBase: base }));
      expect(how).toContain(`href="${ url }"`);
      for (const chain of BRIDGE_CHAINS) expect(how).toContain(chain.label);
      const refusal = renderConsumer(client, createElement(HowItWorks, { explorerBase: 'javascript:alert(1)' }));
      expect(refusal).toContain('Explorer unavailable');
      expect(refusal).not.toContain('href="javascript:');
      expect(refusal).not.toContain('>Paxscan');

      const paused = new ContractFunctionRevertedError({
        abi: layerxBridgeAbi, functionName: 'bridgeIn', message: 'LXP_BRIDGE_PAUSED: paused',
      });
      const error = new BaseError('The bridge write failed', { cause: paused });
      const expected = 'The bridge is paused on this chain. Try again after it reopens.';
      expect(describeWriteError(error)).toBe(expected);
      const id = notifyWalletError('Could not switch network', error);
      expect(toast.getHistory().find((entry) => entry.id === id)).toMatchObject({
        title: 'Could not switch network', description: expected, type: 'error',
      });
      toast.dismiss(id);
      const unknown = new ContractFunctionRevertedError({
        abi: [ { type: 'error', name: 'constructor', inputs: [] } ],
        functionName: 'bridgeIn', data: encodeErrorResult({ abi: [ { type: 'error', name: 'constructor', inputs: [] } ], errorName: 'constructor' }),
      });
      expect(describeWriteError(unknown)).toBe('The contract rejected this: constructor.');
    } finally {
      client.clear();
    }
  }
}

function proofLibAmountRefusalsReachTheCard() {
  for (let observation = 0; observation < 2; observation++) {
    expect(parseAmount('1,234.500001', 6)).toBe(1234500001n);
    expect(bridgeCardAmount('1,234.500001', 6)).toEqual({ parsedAmount: 1234500001n, amountValue: 1234500001n });
    for (const input of [ '1,25', '-1', '1e3', '1.0000001', 'NaN', '' ]) {
      expect(parseAmount(input, 6)).toBeUndefined();
      expect(bridgeCardAmount(input, 6)).toEqual({ parsedAmount: undefined, amountValue: 0n });
    }
    for (const decimals of [ -1, 0.5, NaN, Infinity, 256 ]) {
      expect(parseAmount('1', decimals)).toBeUndefined();
      expect(bridgeCardAmount('1', decimals)).toEqual({ parsedAmount: undefined, amountValue: 0n });
    }
  }
}

async function proofLibLedgerReopensAndRereadsProducerState() {
  const previous = window.localStorage.getItem(storageKey);
  const originalState = wagmiConfig.state;
  try {
    window.localStorage.removeItem(storageKey);
    const input = await signedSubmission();
    let refresh = 0;
    const transfer = recordBridgeSubmission({ direction: 'in', input: { ...input, vault, recipient: wallet.address } }, () => {
      expect(listTransfers(wallet.address)).toHaveLength(1);
      refresh++;
    });
    updateTransfer(transfer.id, { logIndex: '7', nonce: '9' });
    const persisted = window.localStorage.getItem(storageKey);
    expect(persisted).not.toBeNull();
    vi.resetModules();
    const reopened = await import('@/lib/bridge/transfers');
    expect(reopened.listTransfers).not.toBe(listTransfers);
    expect(window.localStorage.getItem(storageKey)).toBe(persisted);
    const connector = wagmiConfig.connectors[0];
    if (!connector) throw new Error('wagmi.ts did not produce an injected connector.');
    wagmiConfig.setState({
      ...originalState, chainId: paxeer.id, current: connector.uid, status: 'connected',
      connections: new Map([ [ connector.uid, { accounts: [ wallet.address ], chainId: paxeer.id, connector } ] ]),
    });
    for (const amount of [ input.amount, input.amount * 2n ]) {
      reopened.updateTransfer(transfer.id, { amount: amount.toString() });
      for (let observation = 0; observation < 2; observation++) {
        const client = newClient();
        try {
          const records = await client.fetchQuery(transferListOptions(wallet.address, refresh, 0));
          expect(records).toEqual(reopened.listTransfers(wallet.address));
          expect(records).toHaveLength(1);
          expect(records[0].amount).toBe(amount.toString());
          const history = transferHistoryRecords(records, []);
          expect(history[0]).toMatchObject({ amount: Number(amount) / 1e18, network: 'Base', status: 'processing', token: 'ETH' });
          const html = renderToStaticMarkup(createElement(TransferHistory, { records: history }));
          expect(html).toContain('Processing');
          expect(html).toContain('ETH');
          const panel = renderConsumer(client, createElement(TransfersPanel, { refreshKey: refresh }));
          expect(panel).toContain('Processing');
          expect(panel).toContain('ETH');
          expect(listTransfers(attestor.address)).toEqual([]);
        } finally {
          client.clear();
        }
      }
    }
    for (const patch of [ { amount: (1n << 256n).toString() }, { logIndex: (1n << 64n).toString() }, { foreignChainId: (1n << 64n).toString() } ]) {
      // Deliberately corrupt a real producer's stored row at its JSON decode boundary.
      window.localStorage.setItem(storageKey, JSON.stringify([ { ...transfer, ...patch } ]));
      const client = newClient();
      try {
        expect(await client.fetchQuery(transferListOptions(wallet.address, refresh, 0))).toEqual([]);
      } finally {
        client.clear();
      }
    }
    expect(refresh).toBe(1);
  } finally {
    wagmiConfig.setState(originalState);
    if (previous === null) window.localStorage.removeItem(storageKey);
    else window.localStorage.setItem(storageKey, previous);
  }
}

async function proofLibHttpAttestationsReachSettlementConsumersWithoutCaching() {
  await libraryHttpBundleReachesReloadedWithdrawalAndStrictThresholdChecks();
}

test(proofLibRegistryExplorerAndErrorsReachRealConsumersTwice.name, proofLibRegistryExplorerAndErrorsReachRealConsumersTwice);
test(proofLibAmountRefusalsReachTheCard.name, proofLibAmountRefusalsReachTheCard);
test(proofLibLedgerReopensAndRereadsProducerState.name, proofLibLedgerReopensAndRereadsProducerState);
test(proofLibHttpAttestationsReachSettlementConsumersWithoutCaching.name, proofLibHttpAttestationsReachSettlementConsumersWithoutCaching);
