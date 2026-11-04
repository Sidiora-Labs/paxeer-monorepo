// @vitest-environment jsdom

import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { act, fireEvent, waitFor } from '@testing-library/react';
import { getEnsNameQueryOptions } from '@wagmi/core/query';
import React from 'react';
import { http } from 'viem';
import { mainnet } from 'viem/chains';
import { createConfig, WagmiProvider } from 'wagmi';

import type { AllowanceType } from 'types/client/revoke';

import { render } from 'ui/paxeerX/account/testWrapper';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';

import Content from './Content';

const ACCOUNT = '0x1111111111111111111111111111111111111111';
const QUERY_KEY = [ 'revoke:approvals', mainnet.id, ACCOUNT ];
const APPROVAL: AllowanceType = {
  type: 'ERC-20',
  address: '0x2222222222222222222222222222222222222222',
  spender: '0x3333333333333333333333333333333333333333',
  transactionId: null,
  allowance: '25',
  symbol: 'SCAN',
  name: 'Recorded approval',
  tokenReputation: 'ok',
  valueAtRiskUsd: 25,
  timestamp: 1624800884000,
};
const walletConfig = createConfig({
  chains: [ mainnet ],
  transports: { [mainnet.id]: http('http://127.0.0.1:1') },
  ssr: true,
});

type ScanSnapshot = { approvals: Array<AllowanceType>; isIncomplete: boolean };

// The scan's peer is an in-memory inventory store. Real React Query observers
// transport its data, terminal errors and refetch lifecycle into production Content.
class ApprovalScanStore {
  reads = 0;
  snapshot: ScanSnapshot | Error;
  private pending?: Promise<void>;
  private release?: () => void;

  constructor(snapshot: ScanSnapshot | Error) {
    this.snapshot = snapshot;
  }

  scan = async(): Promise<ScanSnapshot> => {
    this.reads++;
    await this.pending;
    if (this.snapshot instanceof Error) {
      throw this.snapshot;
    }
    return this.snapshot;
  };

  hold() {
    this.pending = new Promise<void>((resolve) => {
      this.release = resolve;
    });
  }

  resume() {
    this.release?.();
    this.pending = undefined;
    this.release = undefined;
  }
}

const clients: Array<QueryClient> = [];
const stores: Array<ApprovalScanStore> = [];
const originalReact = Object.getOwnPropertyDescriptor(globalThis, 'React');

beforeAll(() => {
  // Vitest's classic JSX transform needs the real runtime for production modules
  // that Next.js normally compiles with the automatic JSX runtime.
  Object.defineProperty(globalThis, 'React', { configurable: true, value: React });
});

afterAll(() => {
  if (originalReact) {
    Object.defineProperty(globalThis, 'React', originalReact);
  } else {
    Reflect.deleteProperty(globalThis, 'React');
  }
});

afterEach(() => {
  stores.splice(0).forEach(store => store.resume());
  clients.splice(0).forEach(client => client.clear());
});

function openInventory(snapshot: ScanSnapshot | Error, withPlaceholder = false) {
  const store = new ApprovalScanStore(snapshot);
  stores.push(store);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity, gcTime: 0 } } });
  clients.push(client);
  // Identity lookups use the real query cache, with a committed no-ENS snapshot;
  // no RPC, wallet, credential or endpoint is needed by this display regression.
  client.setQueryData(getEnsNameQueryOptions(walletConfig, { address: ACCOUNT, chainId: mainnet.id }).queryKey, null);

  function Inventory() {
    const placeholderData = withPlaceholder ? { approvals: [ APPROVAL ], isIncomplete: false } : undefined;
    const snapshotQuery = useQuery({ queryKey: QUERY_KEY, queryFn: store.scan, placeholderData });
    const approvalsQuery = useQuery({ queryKey: QUERY_KEY, queryFn: store.scan, placeholderData, select: (scan) => scan.approvals });

    return React.createElement(Content, {
      searchAddress: ACCOUNT,
      selectedChain: undefined,
      isAddressMatch: false,
      coinBalanceQuery: { isPlaceholderData: false, data: { balance: '0', balanceUsd: '0', symbol: 'ETH', coinImage: undefined } },
      approvalsQuery: { ...approvalsQuery, isIncomplete: snapshotQuery.data?.isIncomplete },
    });
  }

  const view = render(React.createElement(WagmiProvider, { config: walletConfig },
    React.createElement(QueryClientProvider, { client }, React.createElement(Inventory)),
  ));
  return { view, client, store };
}

function expectNoEmptyInventory(view: ReturnType<typeof openInventory>['view']) {
  expect(view.queryByText('No results')).toBeNull();
  expect(view.queryByText(/find any approval/)).toBeNull();
}

async function approvalsErrorDisplayRetriesFailedScan() {
  const failure = new Error('Recorded scan RPC failure');
  const { view, client, store } = openInventory(failure, true);
  await waitFor(() => expect(client.getQueryState(QUERY_KEY)?.status).toBe('error'));
  await view.findByText('Unable to complete the approval scan. Your account may still have active approvals.');
  expect(view.getByText('Something went wrong. Try refreshing the page or come back later.')).toBeTruthy();
  expect(view.getAllByText('Unavailable')).toHaveLength(2);
  expectNoEmptyInventory(view);
  expect(store.reads).toBe(1);

  store.snapshot = { approvals: [], isIncomplete: false };
  store.hold();
  fireEvent.click(view.getByRole('button', { name: 'Retry scan' }));
  await waitFor(() => expect(store.reads).toBe(2));
  const retryButton = await view.findByRole('button', { name: 'Retrying scan' });
  expect((retryButton as HTMLButtonElement).disabled).toBe(true);
  expectNoEmptyInventory(view);
  expect(view.container.querySelectorAll('[data-entity-kind="token"]')).toHaveLength(0);
  expect(view.getAllByText('Unavailable')).toHaveLength(2);
  await act(async() => store.resume());
  await view.findByText('No results');
  expect(client.getQueryState(QUERY_KEY)?.status).toBe('success');
  expect(view.queryByRole('button', { name: 'Retry scan' })).toBeNull();
  expect(view.queryByText('Unavailable')).toBeNull();
  expect(view.getByRole('heading', { name: '0' })).toBeTruthy();
  expect(view.getByRole('heading', { name: '$0' })).toBeTruthy();
}

async function approvalsErrorDisplayRejectsIncompleteEmptyScan() {
  const { view, store } = openInventory({ approvals: [], isIncomplete: true });
  await view.findByText('Unable to complete the approval scan. Your account may still have active approvals.');
  expectNoEmptyInventory(view);
  expect(view.getAllByText('Unavailable')).toHaveLength(2);
  expect(view.getByText('Something went wrong. Try refreshing the page or come back later.')).toBeTruthy();

  store.snapshot = { approvals: [], isIncomplete: false };
  fireEvent.click(view.getByRole('button', { name: 'Retry scan' }));
  await view.findByText('No results');
  expect(store.reads).toBe(2);
  expect(view.queryByText(/Unable to complete/)).toBeNull();
}

async function approvalsErrorDisplayPreservesPartialApprovals() {
  const { view, store } = openInventory({ approvals: [ APPROVAL ], isIncomplete: true });
  await view.findByText('Approval scan is incomplete. Only known approvals are shown; other permissions may still be active.');
  expectNoEmptyInventory(view);
  expect(view.getByRole('heading', { name: 'At least 1' })).toBeTruthy();
  expect(view.getByRole('heading', { name: 'At least $25' })).toBeTruthy();
  const knownTokens = view.container.querySelectorAll('[data-entity-kind="token"]');
  expect(knownTokens).toHaveLength(2);
  knownTokens.forEach(token => expect(token.textContent).toContain(APPROVAL.symbol));

  store.snapshot = { approvals: [ APPROVAL ], isIncomplete: false };
  fireEvent.click(view.getByRole('button', { name: 'Retry scan' }));
  await waitFor(() => expect(view.queryByText(/Approval scan is incomplete/)).toBeNull());
  expect(store.reads).toBe(2);
  expect(view.getByRole('heading', { name: '1' })).toBeTruthy();
  expect(view.getByRole('heading', { name: '$25' })).toBeTruthy();
  expectNoEmptyInventory(view);
}

async function approvalsErrorDisplayPreservesDataAfterRefetchFailure() {
  const { view, client, store } = openInventory({ approvals: [ APPROVAL ], isIncomplete: false });
  await view.findByRole('heading', { name: '1' });
  store.snapshot = new Error('Recorded refetch failure');
  await act(async() => {
    await client.refetchQueries({ queryKey: QUERY_KEY });
  });
  await view.findByText('Approval scan is incomplete. Only known approvals are shown; other permissions may still be active.');
  expect(client.getQueryState(QUERY_KEY)?.status).toBe('error');
  expect(view.getByRole('heading', { name: 'At least 1' })).toBeTruthy();
  expect(view.getByRole('heading', { name: 'At least $25' })).toBeTruthy();
  expectNoEmptyInventory(view);
  expect(view.getByRole('button', { name: 'Retry scan' })).toBeTruthy();

  store.snapshot = { approvals: [ APPROVAL ], isIncomplete: false };
  fireEvent.click(view.getByRole('button', { name: 'Retry scan' }));
  await waitFor(() => expect(view.queryByText(/Approval scan is incomplete/)).toBeNull());
  expect(store.reads).toBe(3);
  expect(view.getByRole('heading', { name: '1' })).toBeTruthy();
}

test(approvalsErrorDisplayRetriesFailedScan.name, approvalsErrorDisplayRetriesFailedScan);
test(approvalsErrorDisplayRejectsIncompleteEmptyScan.name, approvalsErrorDisplayRejectsIncompleteEmptyScan);
test(approvalsErrorDisplayPreservesPartialApprovals.name, approvalsErrorDisplayPreservesPartialApprovals);
test(approvalsErrorDisplayPreservesDataAfterRefetchFailure.name, approvalsErrorDisplayPreservesDataAfterRefetchFailure);
