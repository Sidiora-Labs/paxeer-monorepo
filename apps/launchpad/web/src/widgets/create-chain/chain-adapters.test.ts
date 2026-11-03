import { expect, test, vi } from 'vitest';
import {
  ContractFunctionExecutionError, ContractFunctionRevertedError, createPublicClient, createWalletClient, custom,
  decodeFunctionResult, defineChain, encodeAbiParameters, encodeErrorResult, encodeEventTopics, encodeFunctionData,
  parseAbi, type Hex, type PublicClient, type TransactionReceipt, type WalletClient,
} from 'viem';
import { ChainLaunchAdapter, chainsAbi, type ChainLaunchStep } from './chain-adapters';
import { CHAINS_PRECOMPILE, chainCost, type ChainDraft } from './chain-domain';
import vectors from '../../../../../../tests/vectors/chains/abi-vectors.json';

const creator = '0x1111111111111111111111111111111111111111';
const other = '0x2222222222222222222222222222222222222222';
const txHash: Hex = `0x${'ab'.repeat(32)}`;
const blockHash: Hex = `0x${'cd'.repeat(32)}`;
const chain = defineChain({
  id: 125, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
  rpcUrls: { default: { http: ['https://rpc.example/evm'] } },
  blockExplorers: { default: { name: 'Explorer', url: 'https://explorer.example' } },
});

function unknownChainError() {
  return new ContractFunctionExecutionError(new ContractFunctionRevertedError({
    abi: chainsAbi, functionName: 'getChain',
    data: encodeErrorResult({
      abi: parseAbi(['error Error(string)']), errorName: 'Error', args: ['LXP_UNKNOWN_FIELD: unknown domain'],
    }),
  }), { abi: chainsAbi, functionName: 'getChain', args: [1], contractAddress: CHAINS_PRECOMPILE });
}

function chainFixture() {
  const draft: ChainDraft = {
    draftId: 'chain-demo', version: 1, creator, chainId: 125, name: 'demo',
    tier: 'starter', feeShareBps: 100, updatedAt: Date.now(),
  };
  const scope = { creator, chainId: 125, epoch: 1 };
  const params = decodeFunctionResult({ abi: chainsAbi, functionName: 'params', data: vectors.params.returndata as Hex });
  const record = decodeFunctionResult({ abi: chainsAbi, functionName: 'getChain', data: vectors.getChain.returndata as Hex });
  const eventAbi = parseAbi(['event ChainCreated(uint32 indexed id,string name,address indexed operator,uint64 chainId)']);
  const log: TransactionReceipt['logs'][number] = {
    address: CHAINS_PRECOMPILE,
    topics: encodeEventTopics({ abi: eventAbi, eventName: 'ChainCreated', args: { id: 1, operator: creator } }) as [Hex, ...Hex[]],
    data: encodeAbiParameters([{ type: 'string' }, { type: 'uint64' }], ['demo', 125000001n]),
    blockHash, blockNumber: 42n, transactionHash: txHash, transactionIndex: 0, logIndex: 0, removed: false,
  };
  const receipt: TransactionReceipt = {
    status: 'success', transactionHash: txHash, blockHash, blockNumber: 42n, transactionIndex: 0,
    from: creator, to: CHAINS_PRECOMPILE, contractAddress: null, cumulativeGasUsed: 60000n,
    gasUsed: 60000n, effectiveGasPrice: 1000000000n, logsBloom: `0x${'00'.repeat(256)}`,
    type: 'eip1559', logs: [log],
  };
  const transport = custom({ request: async () => { throw new Error('Unexpected RPC request'); } });
  const publicClient = createPublicClient({ chain, transport }) as PublicClient;
  const walletClient = createWalletClient({ chain, transport }) as WalletClient;
  const readContract = vi.spyOn(publicClient, 'readContract').mockImplementation(async request => {
    if (request.address !== CHAINS_PRECOMPILE) throw new Error('Unexpected registry address');
    switch (request.functionName) {
      case 'params': return params;
      case 'chainByName': return 0;
      case 'getChain': return record;
      default: throw new Error('Unexpected registry method');
    }
  });
  const client = {
    chain, readContract,
    getChainId: vi.spyOn(publicClient, 'getChainId').mockResolvedValue(125),
    estimateContractGas: vi.spyOn(publicClient, 'estimateContractGas').mockResolvedValue(60000n),
    getGasPrice: vi.spyOn(publicClient, 'getGasPrice').mockResolvedValue(1000000000n),
    waitForTransactionReceipt: vi.spyOn(publicClient, 'waitForTransactionReceipt').mockResolvedValue(receipt),
  };
  const wallet = {
    chain,
    getChainId: vi.spyOn(walletClient, 'getChainId').mockResolvedValue(125),
    getAddresses: vi.spyOn(walletClient, 'getAddresses').mockResolvedValue([creator]),
    writeContract: vi.spyOn(walletClient, 'writeContract').mockResolvedValue(txHash),
  };
  const adapter = new ChainLaunchAdapter(publicClient, walletClient, () => scope);
  return { adapter, draft, scope, client, wallet, params, record, receipt, log };
}

async function chain_adapter_launches_and_reports_the_new_chain() {
  const f = chainFixture();
  expect(await f.adapter.isNameAvailable('demo')).toBe(true);
  expect(f.client.readContract).toHaveBeenLastCalledWith({
    address: CHAINS_PRECOMPILE, abi: chainsAbi, functionName: 'chainByName', args: ['demo'],
  });
  expect(await f.adapter.readParams()).toEqual(f.params);
  const cost = chainCost(f.params, f.draft.tier, '60000000000000');
  expect(cost).toEqual({
    bondRaw: vectors.params.decoded.minBond, feeRaw: vectors.params.decoded.creationFee,
    totalValueRaw: '101000000000000000000', gasEstimatePaxRaw: '60000000000000',
  });
  const read = f.client.readContract.getMockImplementation()!;
  let polls = 0;
  f.client.readContract.mockImplementation(async request => {
    if (request.functionName === 'getChain' && ++polls === 1) throw unknownChainError();
    return read(request);
  });
  const steps: ChainLaunchStep[] = [];
  const result = await f.adapter.launch(f.draft, step => steps.push(step));
  expect(steps).toEqual(['create', 'confirm', 'ready']);
  expect(f.wallet.writeContract).toHaveBeenCalledTimes(1);
  const request = f.wallet.writeContract.mock.calls[0][0];
  expect(request).toEqual({
    address: CHAINS_PRECOMPILE, abi: chainsAbi, functionName: 'createChain',
    args: ['demo', 500000n, 65536n, 100], value: BigInt(cost.totalValueRaw), account: creator, chain,
  });
  expect(encodeFunctionData(request)).toBe(vectors.methods.find(method => method.name === 'createChain')!.calldata);
  expect(f.client.estimateContractGas).toHaveBeenCalledWith({
    address: CHAINS_PRECOMPILE, abi: chainsAbi, functionName: 'createChain',
    args: ['demo', 500000n, 65536n, 100], value: BigInt(cost.totalValueRaw), account: creator,
  });
  expect(f.client.waitForTransactionReceipt).toHaveBeenCalledWith({ hash: txHash, timeout: 30000 });
  expect(f.log.topics[0]).toBe(vectors.events.find(event => event.name === 'ChainCreated')!.topic0);
  expect(polls).toBe(2);
  expect(f.client.readContract).toHaveBeenLastCalledWith({
    address: CHAINS_PRECOMPILE, abi: chainsAbi, functionName: 'getChain', args: [1], blockNumber: 42n,
  });
  expect(result).toEqual({
    id: 1, chainId: 125000001, name: 'demo', txHash,
    rpcUrl: 'https://rpc.example/evm/chain/1', explorerUrl: 'https://explorer.example/chains/1',
    walletNetwork: {
      chainId: '0x7735941', chainName: 'demo on Paxeer',
      rpcUrls: ['https://rpc.example/evm/chain/1'],
      nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
      blockExplorerUrls: ['https://explorer.example/chains/1'],
    },
  });
  for (const tier of ['growth', 'pro'] as const) {
    const sized = chainFixture();
    const allowance = tier === 'growth' ? [1500000n, 262144n] as const : [4000000n, 1048576n] as const;
    const sizedRead = sized.client.readContract.getMockImplementation()!;
    sized.client.readContract.mockImplementation(async request => request.functionName === 'getChain'
      ? [1, 'demo', creator, 1, 125000001n, ...allowance, 100, sized.params[1], 1000000000n, 42n]
      : sizedRead(request));
    await sized.adapter.launch({ ...sized.draft, tier }, () => {});
    expect(sized.wallet.writeContract.mock.calls[0][0].args).toEqual(['demo', ...allowance, 100]);
  }
}

async function chain_adapter_refuses_a_taken_name_and_a_changed_wallet() {
  const taken = chainFixture();
  const takenRead = taken.client.readContract.getMockImplementation()!;
  taken.client.readContract.mockImplementation(async request => request.functionName === 'chainByName' ? 1 : takenRead(request));
  expect(await taken.adapter.isNameAvailable('demo')).toBe(false);
  await expect(taken.adapter.launch(taken.draft, () => {})).rejects.toThrow('Chain name is taken');
  expect(taken.wallet.writeContract).not.toHaveBeenCalled();

  const changed = chainFixture();
  const expected = changed.adapter.scope();
  changed.scope.epoch++;
  expect(() => changed.adapter.scope(expected)).toThrow('Wallet changed');
  await expect(changed.adapter.assertWallet(expected)).rejects.toThrow('Wallet changed');
  const changedRead = changed.client.readContract.getMockImplementation()!;
  changed.client.readContract.mockImplementation(async request => {
    const result = await changedRead(request);
    if (request.functionName === 'params') changed.scope.epoch++;
    return result;
  });
  await expect(changed.adapter.launch(changed.draft, () => {})).rejects.toThrow('Wallet changed');
  expect(changed.wallet.writeContract).not.toHaveBeenCalled();

  for (const kind of ['account', 'wallet-chain', 'rpc-chain', 'creator', 'invalid-draft', 'capacity'] as const) {
    const f = chainFixture();
    let error: string;
    switch (kind) {
      case 'account': f.wallet.getAddresses.mockResolvedValue([other]); error = 'Wallet account or chain changed'; break;
      case 'wallet-chain': f.wallet.getChainId.mockResolvedValue(1); error = 'Switch to chain 125'; break;
      case 'rpc-chain': f.client.getChainId.mockResolvedValue(1); error = 'Switch to chain 125'; break;
      case 'creator': f.draft.creator = other; error = 'Draft creator changed'; break;
      case 'invalid-draft': f.draft.name = 'ABC'; error = 'Invalid chain fields'; break;
      case 'capacity': {
        const read = f.client.readContract.getMockImplementation()!;
        f.client.readContract.mockImplementation(async request => request.functionName === 'params'
          ? [creator, f.params[1], f.params[2], f.params[3], f.params[4], 499999n, f.params[6]]
          : read(request));
        error = 'tier_unavailable'; break;
      }
    }
    await expect(f.adapter.launch(f.draft, () => {})).rejects.toThrow(error);
    expect(f.wallet.writeContract).not.toHaveBeenCalled();
  }

  for (const kind of ['reverted', 'missing-event', 'wrong-event', 'wrong-record', 'rpc-error'] as const) {
    const f = chainFixture();
    const steps: ChainLaunchStep[] = [];
    let error: string;
    switch (kind) {
      case 'reverted': f.receipt.status = 'reverted'; error = 'Chain creation receipt failed'; break;
      case 'missing-event': f.receipt.logs = []; error = 'Expected one ChainCreated'; break;
      case 'wrong-event': {
        f.log.data = encodeAbiParameters([{ type: 'string' }, { type: 'uint64' }], ['other', 125000001n]);
        error = 'ChainCreated event does not match'; break;
      }
      case 'wrong-record': {
        const read = f.client.readContract.getMockImplementation()!;
        f.client.readContract.mockImplementation(async request => request.functionName === 'getChain'
          ? [1, 'other', ...f.record.slice(2)] : read(request));
        error = 'Chain registry record does not match'; break;
      }
      case 'rpc-error': {
        const read = f.client.readContract.getMockImplementation()!;
        f.client.readContract.mockImplementation(async request => {
          if (request.functionName === 'getChain') throw new Error('Registry unavailable');
          return read(request);
        });
        error = 'Registry unavailable'; break;
      }
    }
    const outcome = await f.adapter.launch(f.draft, step => steps.push(step));
    expect(outcome).toMatchObject({ txHash, submission: { txHash, phase: kind === 'reverted' ? 'reverted' : 'confirm' } });
    if (!('error' in outcome)) throw new Error('Expected a confirmation outcome');
    expect(outcome.error).toContain(error);
    expect(steps).not.toContain('ready');
  }

  vi.useFakeTimers();
  try {
    const missing = chainFixture();
    const read = missing.client.readContract.getMockImplementation()!;
    missing.client.readContract.mockImplementation(async request => {
      if (request.functionName === 'getChain') throw unknownChainError();
      return read(request);
    });
    const confirmation = missing.adapter.launch(missing.draft, () => {});
    await vi.runAllTimersAsync();
    expect(await confirmation).toMatchObject({ txHash, submission: { phase: 'confirm' }, error: 'Chain registry record is unavailable after confirmation.' });
    expect(missing.client.readContract.mock.calls.filter(([request]) => request.functionName === 'getChain')).toHaveLength(30);
    expect(missing.wallet.writeContract).toHaveBeenCalledTimes(1);
  } finally {
    vi.useRealTimers();
  }
}

async function chain_adapter_recovers_receipt_and_readback_without_resubmitting() {
  for (const interruption of ['receipt', 'readback'] as const) {
    const f = chainFixture();
    if (interruption === 'receipt') f.client.waitForTransactionReceipt.mockRejectedValueOnce(new Error('Receipt timeout'));
    else {
      const read = f.client.readContract.getMockImplementation()!;
      let interrupted = false;
      f.client.readContract.mockImplementation(async request => {
        if (request.functionName === 'getChain' && !interrupted) {
          interrupted = true;
          throw new Error('Readback interrupted');
        }
        return read(request);
      });
    }
    let observedHash: Hex | undefined;
    const outcome = await f.adapter.launch(f.draft, (_step, submission) => {
      if (submission) observedHash = submission.txHash;
    });
    expect(observedHash).toBe(txHash);
    if (!('submission' in outcome)) throw new Error('Expected interrupted confirmation');
    expect(outcome.submission).toEqual({ creator, chainId: 125, txHash, phase: 'confirm', bondRaw: f.params[1].toString() });
    expect(outcome.error).toContain(interruption === 'receipt' ? 'Receipt timeout' : 'Readback interrupted');
    await expect(f.adapter.launch(f.draft, () => {})).rejects.toThrow('Retry confirmation');
    const recovered = await f.adapter.confirm(txHash, f.draft, outcome.submission);
    expect(recovered).toMatchObject({ id: 1, chainId: 125000001, txHash });
    expect(f.wallet.writeContract).toHaveBeenCalledTimes(1);
    expect(f.client.waitForTransactionReceipt.mock.calls.every(([request]) => request.hash === txHash)).toBe(true);
  }
}

async function chain_adapter_distinguishes_revert_from_unknown_and_checks_recovery_scope() {
  const f = chainFixture();
  f.client.waitForTransactionReceipt.mockRejectedValueOnce(new Error('Receipt timeout'));
  const outcome = await f.adapter.launch(f.draft, () => {});
  if (!('submission' in outcome)) throw new Error('Expected interrupted confirmation');
  f.scope.creator = other;
  const changed = await f.adapter.confirm(txHash, f.draft, outcome.submission);
  expect(changed).toMatchObject({ submission: { phase: 'confirm', creator }, error: 'Submitted transaction belongs to a different account or chain.' });
  expect(f.client.waitForTransactionReceipt).toHaveBeenCalledTimes(1);
  f.scope.creator = creator;
  f.receipt.status = 'reverted';
  f.receipt.transactionHash = blockHash;
  expect(await f.adapter.confirm(txHash, f.draft, outcome.submission)).toMatchObject({ submission: { phase: 'confirm' }, error: 'Chain creation receipt failed.' });
  f.receipt.transactionHash = txHash;
  expect(await f.adapter.confirm(txHash, f.draft, outcome.submission)).toMatchObject({ txHash, submission: { phase: 'reverted' } });
  const read = f.client.readContract.getMockImplementation()!;
  f.client.readContract.mockImplementation(async request => request.functionName === 'chainByName' ? 1 : read(request));
  await expect(f.adapter.launch(f.draft, () => {})).rejects.toThrow('Chain name is taken');
  expect(f.wallet.writeContract).toHaveBeenCalledTimes(1);
  f.client.readContract.mockImplementation(read);
  f.receipt.status = 'success';
  expect(await f.adapter.launch(f.draft, () => {})).toMatchObject({ id: 1, txHash });
  expect(f.wallet.writeContract).toHaveBeenCalledTimes(2);
}

test(chain_adapter_launches_and_reports_the_new_chain);
test(chain_adapter_refuses_a_taken_name_and_a_changed_wallet);
test(chain_adapter_recovers_receipt_and_readback_without_resubmitting);
test(chain_adapter_distinguishes_revert_from_unknown_and_checks_recovery_scope);
