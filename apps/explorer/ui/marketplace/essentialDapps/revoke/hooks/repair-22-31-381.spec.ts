import NftArtifact from '@openzeppelin/contracts/build/contracts/ERC721.json';
import type { Address, Hex, PublicClient, RpcLog, RpcTransactionReceipt } from 'viem';
import { createPublicClient, createWalletClient, custom, parseAbiItem, toEventSelector, toHex } from 'viem';
import { foundry } from 'viem/chains';

import type { AllowanceType } from 'types/client/revoke';

import { ZERO_ADDRESS } from 'toolkit/utils/consts';
import { afterEach, beforeEach, expect, test } from 'vitest';

import { getRevokeContractParameters, verifyNftRevocation } from './revokeApproval';

// Task 22.31.440 fixture source: the MyNFT approval/read/error shapes exercised by
// repair 22.31.381 against contracts/src/ERC721.sol. Replay uses ABI words and
// explicit block snapshots, never computes permissions from the hook's writes.
// Kept: both token IDs, operator isolation, both mismatched-receipt refusals,
// historical committed-block reads after reapproval, and the burned-token error.
// Baseline verify was not run: the ordered gates forbid verification before
// implementation; qualification.kvx finding 22.31.405.1 records the import failure.
const owner: Address = '0x90F8bf6A479f320ead074411a4B0e7944Ea8c9C1';
const spender: Address = '0xFFcf8FDEE72ac11b5c542428B35EEF5769C409f0';
const address: Address = '0xe78A0F7E598Cc8b0Bb87894B0F60dD2a88d6a8Ab';
const artifact = { abi: [ parseAbiItem('function burn(uint256 id)') ] };
const zeroWord = '0000000000000000000000000000000000000000000000000000000000000000';
const oneWord = '0000000000000000000000000000000000000000000000000000000000000001';
const seventeenWord = '0000000000000000000000000000000000000000000000000000000000000011';
const spenderWord = '000000000000000000000000ffcf8fdee72ac11b5c542428b35eef5769c409f0';
const ownerWord = '00000000000000000000000090f8bf6a479f320ead074411a4b0e7944ea8c9c1';
const approvedResult: Hex = `0x${ spenderWord }`;
const clearedResult: Hex = `0x${ zeroWord }`;
const trueResult: Hex = `0x${ oneWord }`;
const tokenZeroCall: Hex = `0x081812fc${ zeroWord }`;
const tokenSeventeenCall: Hex = `0x081812fc${ seventeenWord }`;
const operatorCall: Hex = `0xe985e9c5${ ownerWord }${ spenderWord }`;
const clearZeroData: Hex = `0x095ea7b3${ zeroWord }${ zeroWord }`;
const clearSeventeenData: Hex = `0x095ea7b3${ zeroWord }${ seventeenWord }`;
const clearOperatorData: Hex = `0xa22cb465${ spenderWord }${ zeroWord }`;
const approveSeventeenData: Hex = `0x095ea7b3${ spenderWord }${ seventeenWord }`;
const approveOperatorData: Hex = `0xa22cb465${ spenderWord }${ oneWord }`;
const burnSeventeenData: Hex = `0x42966c68${ seventeenWord }`;
const burnedTokenError = {
  code: 3,
  message: 'VM Exception while processing transaction: revert token doesn\'t exist',
  data: '0x08c379a0' +
    '0000000000000000000000000000000000000000000000000000000000000020' +
    '0000000000000000000000000000000000000000000000000000000000000013' +
    '746f6b656e20646f65736e277420657869737400000000000000000000000000',
};

type ReadResult = Hex | typeof burnedTokenError;
type RecordedReads = readonly [ReadResult, ReadResult, Hex];
type RecordedWrite = { data: Hex; reads: RecordedReads };
const initialReads: RecordedReads = [ approvedResult, approvedResult, trueResult ];
const tokenZeroCleared: RecordedReads = [ clearedResult, approvedResult, trueResult ];
const tokenSeventeenCleared: RecordedReads = [ approvedResult, clearedResult, trueResult ];
const operatorCleared: RecordedReads = [ approvedResult, approvedResult, clearedResult ];
const recordings: Record<string, ReadonlyArray<RecordedWrite>> = {
  nftTokenRevokeClearsOnlySelectedToken: [
    { data: clearZeroData, reads: tokenZeroCleared },
    { data: clearSeventeenData, reads: [ clearedResult, clearedResult, trueResult ] },
  ],
  nftOperatorRevokeClearsOnlyCollectionPermission: [
    { data: clearOperatorData, reads: operatorCleared },
  ],
  nftTokenRevokeRejectsSuccessfulOperatorOnlyReceipt: [
    { data: clearOperatorData, reads: operatorCleared },
  ],
  nftOperatorRevokeRejectsSuccessfulTokenOnlyReceipt: [
    { data: clearSeventeenData, reads: tokenSeventeenCleared },
  ],
  nftRevocationChecksTheCommittedBlock: [
    { data: clearSeventeenData, reads: tokenSeventeenCleared },
    { data: approveSeventeenData, reads: initialReads },
    { data: clearOperatorData, reads: operatorCleared },
    { data: approveOperatorData, reads: initialReads },
  ],
  nftRevocationPropagatesReadFailure: [
    { data: burnSeventeenData, reads: [ approvedResult, burnedTokenError, trueResult ] },
  ],
};

function fixtureHash(value: number): Hex {
  return toHex(value, { size: 32 });
}

function recordedLog(kind: 'Approval' | 'ApprovalForAll', tokenId: bigint, block: number): RpcLog {
  const topics: [Hex, ...Array<Hex>] = kind === 'Approval' ? [
    toEventSelector('Approval(address,address,uint256)'),
    `0x${ ownerWord }`,
    `0x${ spenderWord }`,
    toHex(tokenId, { size: 32 }),
  ] : [
    toEventSelector('ApprovalForAll(address,address,bool)'),
    `0x${ ownerWord }`,
    `0x${ spenderWord }`,
  ];
  return {
    address,
    topics,
    data: kind === 'Approval' ? '0x' : trueResult,
    blockNumber: toHex(block),
    blockHash: fixtureHash(block),
    transactionHash: fixtureHash(100 + block),
    transactionIndex: '0x0',
    logIndex: '0x0',
    removed: false,
  };
}

const approvalLogs = [
  recordedLog('Approval', BigInt(0), 3),
  recordedLog('Approval', BigInt(17), 5),
  recordedLog('ApprovalForAll', BigInt(0), 6),
];

function startRpcFixture(writes: ReadonlyArray<RecordedWrite>) {
  let cursor = 0;
  let stopped = false;
  const initialBlock = 6;
  const snapshots = new Map<Hex, RecordedReads>([ [ toHex(initialBlock), initialReads ] ]);
  const receipts = new Map<Hex, RpcTransactionReceipt>();

  return {
    async request({ method, params = [] }: { method: string; params?: ReadonlyArray<unknown> }) {
      expect(stopped).toBe(false);
      if (method === 'eth_accounts') {
        return [ owner, spender ];
      }
      if (method === 'eth_chainId') {
        return toHex(foundry.id);
      }
      if (method === 'eth_blockNumber') {
        return toHex(initialBlock + cursor);
      }
      if (method === 'eth_sendTransaction') {
        const [ transaction ] = params as [ { from: Address; to: Address; data: Hex } ];
        const next = writes[cursor];
        expect(next, 'Unexpected transaction after the recorded writes').toBeDefined();
        expect(transaction.from.toLowerCase()).toBe(owner.toLowerCase());
        expect(transaction.to.toLowerCase()).toBe(address.toLowerCase());
        expect(transaction.data.toLowerCase()).toBe(next.data);
        cursor++;
        const block = initialBlock + cursor;
        const hash = fixtureHash(100 + block);
        snapshots.set(toHex(block), next.reads);
        receipts.set(hash, {
          transactionHash: hash,
          transactionIndex: '0x0',
          blockHash: fixtureHash(block),
          blockNumber: toHex(block),
          from: owner,
          to: address,
          contractAddress: null,
          cumulativeGasUsed: '0x10000',
          gasUsed: '0x10000',
          effectiveGasPrice: '0x3b9aca00',
          logs: [],
          logsBloom: `0x${ '00'.repeat(256) }`,
          status: '0x1',
          type: '0x2',
        });
        return hash;
      }
      if (method === 'eth_getTransactionReceipt') {
        const receipt = receipts.get(params[0] as Hex);
        expect(receipt, 'Receipt requested for an unrecorded transaction').toBeDefined();
        return receipt;
      }
      if (method === 'eth_call') {
        const [ call, block ] = params as [ { to: Address; data: Hex }, Hex | 'latest' ];
        expect(call.to.toLowerCase()).toBe(address.toLowerCase());
        const reads = snapshots.get(block === 'latest' ? toHex(initialBlock + cursor) : block);
        expect(reads, 'Read requested at an unrecorded block').toBeDefined();
        const index = [ tokenZeroCall, tokenSeventeenCall, operatorCall ].indexOf(call.data.toLowerCase() as Hex);
        expect(index, 'Unrecorded eth_call calldata').toBeGreaterThanOrEqual(0);
        const result = reads![index];
        if (typeof result !== 'string') {
          throw result;
        }
        return result;
      }
      if (method === 'eth_getLogs') {
        const [ filter ] = params as [ { address: Address; fromBlock: Hex; toBlock: string; topics: Array<Hex | null> } ];
        expect(filter.address.toLowerCase()).toBe(address.toLowerCase());
        expect(filter.fromBlock).toBe('0x0');
        expect(filter.toBlock).toBe('latest');
        const logs = approvalLogs.filter((log) => filter.topics.every((topic, index) =>
          topic === null || topic.toLowerCase() === log.topics[index]?.toLowerCase(),
        ));
        expect(logs.length, 'No recorded approval matches the requested topics').toBeGreaterThan(0);
        return logs;
      }
      if (method === 'eth_getCode') {
        // The recorded EOA has no code; no contract bytecode is needed for replay.
        expect((params[0] as Address).toLowerCase()).toBe(owner.toLowerCase());
        return '0x';
      }
      throw new Error(`Unrecorded JSON-RPC method: ${ method }`);
    },
    stop() {
      stopped = true;
      expect(cursor, 'Every recorded write must execute').toBe(writes.length);
    },
  };
}

let provider: ReturnType<typeof startRpcFixture>;
let publicClient: PublicClient;
let walletClient: ReturnType<typeof createWalletClient>;

async function commit(hash: Hex) {
  const receipt = await publicClient.waitForTransactionReceipt({ hash });
  expect(receipt.status).toBe('success');
  return receipt;
}

beforeEach(async({ task }) => {
  const writes = recordings[task.name];
  expect(writes, 'Every test must select an explicit recording').toBeDefined();
  provider = startRpcFixture(writes);
  publicClient = createPublicClient({ transport: custom(provider, { retryCount: 0 }), cacheTime: 0 });
  walletClient = createWalletClient({ chain: foundry, transport: custom(provider, { retryCount: 0 }) });
  expect(await walletClient.getAddresses()).toEqual([ owner, spender ]);
});

afterEach(() => {
  provider?.stop();
});

async function tokenRow(tokenId: bigint): Promise<AllowanceType> {
  const events = await publicClient.getContractEvents({
    address,
    abi: [ parseAbiItem('event Approval(address indexed owner, address indexed spender, uint256 indexed id)') ],
    eventName: 'Approval',
    args: { owner, id: tokenId },
    fromBlock: BigInt(0),
    toBlock: 'latest',
  });
  expect(events.length).toBeGreaterThan(0);
  const event = events[events.length - 1];
  const approved = await publicClient.readContract({
    address,
    abi: NftArtifact.abi,
    functionName: 'getApproved',
    args: [ tokenId ],
  });
  expect(approved).toBe(spender);
  expect(event.args.spender).toBe(spender);

  return {
    type: 'ERC-721',
    address,
    spender,
    tokenId: toHex(event.args.id!, { size: 32 }),
    transactionId: event.transactionHash,
    tokenReputation: null,
    timestamp: 0,
  };
}

async function operatorRow(): Promise<AllowanceType> {
  const events = await publicClient.getContractEvents({
    address,
    abi: [ parseAbiItem('event ApprovalForAll(address indexed owner, address indexed operator, bool approved)') ],
    eventName: 'ApprovalForAll',
    args: { owner, operator: spender },
    fromBlock: BigInt(0),
    toBlock: 'latest',
  });
  expect(events.length).toBeGreaterThan(0);
  const event = events[events.length - 1];
  expect(event.args.approved).toBe(true);
  expect(await operatorPermission()).toBe(true);

  return {
    type: 'ERC-721',
    address,
    spender: event.args.operator!,
    transactionId: event.transactionHash,
    tokenReputation: null,
    timestamp: 0,
  };
}

async function tokenPermission(tokenId: bigint) {
  return publicClient.readContract({ address, abi: NftArtifact.abi, functionName: 'getApproved', args: [ tokenId ] });
}

async function operatorPermission() {
  return publicClient.readContract({ address, abi: NftArtifact.abi, functionName: 'isApprovedForAll', args: [ owner, spender ] });
}

async function revoke(approval: AllowanceType) {
  const receipt = await commit(await walletClient.writeContract({
    ...getRevokeContractParameters(approval),
    account: owner,
    chain: foundry,
  }));
  await verifyNftRevocation(approval, owner, receipt.blockNumber, publicClient);
  return receipt;
}

async function nftTokenRevokeClearsOnlySelectedToken() {
  for (const tokenId of [ BigInt(0), BigInt(17) ]) {
    const approval = await tokenRow(tokenId);
    await revoke(approval);
    expect(await tokenPermission(tokenId)).toBe(ZERO_ADDRESS);
    expect(await operatorPermission()).toBe(true);
    if (tokenId === BigInt(0)) {
      expect(await tokenPermission(BigInt(17))).toBe(spender);
    }
  }
}

async function nftOperatorRevokeClearsOnlyCollectionPermission() {
  await revoke(await operatorRow());
  expect(await operatorPermission()).toBe(false);
  expect(await tokenPermission(BigInt(0))).toBe(spender);
  expect(await tokenPermission(BigInt(17))).toBe(spender);
}

async function nftTokenRevokeRejectsSuccessfulOperatorOnlyReceipt() {
  const approval = await tokenRow(BigInt(17));
  const receipt = await commit(await walletClient.writeContract({
    address,
    abi: NftArtifact.abi,
    functionName: 'setApprovalForAll',
    args: [ spender, false ],
    account: owner,
    chain: foundry,
  }));
  await expect(verifyNftRevocation(approval, owner, receipt.blockNumber, publicClient))
    .rejects.toThrow('Token approval remains active');
  expect(await tokenPermission(BigInt(17))).toBe(spender);
}

async function nftOperatorRevokeRejectsSuccessfulTokenOnlyReceipt() {
  const approval = await operatorRow();
  const receipt = await revoke(await tokenRow(BigInt(17)));
  await expect(verifyNftRevocation(approval, owner, receipt.blockNumber, publicClient))
    .rejects.toThrow('Operator approval remains active');
  expect(await operatorPermission()).toBe(true);
}

async function nftRevocationChecksTheCommittedBlock() {
  const tokenApproval = await tokenRow(BigInt(17));
  const tokenReceipt = await revoke(tokenApproval);
  await commit(await walletClient.writeContract({
    address,
    abi: NftArtifact.abi,
    functionName: 'approve',
    args: [ spender, BigInt(17) ],
    account: owner,
    chain: foundry,
  }));
  expect(await tokenPermission(BigInt(17))).toBe(spender);
  await expect(verifyNftRevocation(tokenApproval, owner, tokenReceipt.blockNumber, publicClient)).resolves.toBeUndefined();
  await expect(verifyNftRevocation(tokenApproval, owner, await publicClient.getBlockNumber(), publicClient))
    .rejects.toThrow('Token approval remains active');

  const operatorApproval = await operatorRow();
  const operatorReceipt = await revoke(operatorApproval);
  await commit(await walletClient.writeContract({
    address,
    abi: NftArtifact.abi,
    functionName: 'setApprovalForAll',
    args: [ spender, true ],
    account: owner,
    chain: foundry,
  }));
  expect(await operatorPermission()).toBe(true);
  await expect(verifyNftRevocation(operatorApproval, owner, operatorReceipt.blockNumber, publicClient)).resolves.toBeUndefined();
  await expect(verifyNftRevocation(operatorApproval, owner, await publicClient.getBlockNumber(), publicClient))
    .rejects.toThrow('Operator approval remains active');
}

async function nftRevocationPropagatesReadFailure() {
  const approval = await tokenRow(BigInt(17));
  await commit(await walletClient.writeContract({
    address,
    abi: artifact.abi,
    functionName: 'burn',
    args: [ BigInt(17) ],
    account: owner,
    chain: foundry,
  }));
  await expect(verifyNftRevocation(approval, owner, await publicClient.getBlockNumber(), publicClient))
    .rejects.toThrow('token doesn\'t exist');
}

test(nftTokenRevokeClearsOnlySelectedToken);
test(nftOperatorRevokeClearsOnlyCollectionPermission);
test(nftTokenRevokeRejectsSuccessfulOperatorOnlyReceipt);
test(nftOperatorRevokeRejectsSuccessfulTokenOnlyReceipt);
test(nftRevocationChecksTheCommittedBlock);
test(nftRevocationPropagatesReadFailure);
