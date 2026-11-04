import NftArtifact from '@openzeppelin/contracts/build/contracts/ERC721.json';
import ganache from 'ganache';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import type { Abi, Address, Hex, PublicClient } from 'viem';
import { createPublicClient, createWalletClient, custom, parseAbiItem, toHex } from 'viem';
import { foundry } from 'viem/chains';

import type { AllowanceType } from 'types/client/revoke';

import { ZERO_ADDRESS } from 'toolkit/utils/consts';
import { afterEach, beforeAll, beforeEach, expect, test } from 'vitest';

import { getRevokeContractParameters, verifyNftRevocation } from './revokeApproval';

const solc: { compile: (input: string) => string } = createRequire(import.meta.url)('solc');
let artifact: { abi: Abi; evm: { bytecode: { object: string } } };
let provider: ReturnType<typeof ganache.provider>;
let publicClient: PublicClient;
let walletClient: ReturnType<typeof createWalletClient>;
let owner: Address;
let spender: Address;
let address: Address;

beforeAll(() => {
  const source = readFileSync(resolve(__dirname, '../../../../../../../contracts/src/ERC721.sol'), 'utf8');
  const output = JSON.parse(solc.compile(JSON.stringify({
    language: 'Solidity',
    sources: { 'ERC721.sol': { content: source } },
    settings: {
      evmVersion: 'shanghai',
      outputSelection: { '*': { '*': [ 'abi', 'evm.bytecode.object' ] } },
    },
  }))) as {
    contracts: Record<string, Record<string, typeof artifact>>;
    errors?: Array<{ severity: string; formattedMessage: string }>;
  };

  expect(output.errors?.filter((error) => error.severity === 'error') ?? []).toEqual([]);
  artifact = output.contracts['ERC721.sol'].MyNFT;
}, 30_000);

async function commit(hash: Hex) {
  const receipt = await publicClient.waitForTransactionReceipt({ hash });
  expect(receipt.status).toBe('success');
  return receipt;
}

beforeEach(async() => {
  provider = ganache.provider({
    chain: { chainId: foundry.id, hardfork: 'shanghai' },
    wallet: { deterministic: true, totalAccounts: 3 },
    logging: { quiet: true },
  });
  publicClient = createPublicClient({ transport: custom(provider), cacheTime: 0 });
  walletClient = createWalletClient({ chain: foundry, transport: custom(provider) });
  [ owner, spender ] = await walletClient.getAddresses();

  const deployed = await commit(await walletClient.deployContract({
    account: owner,
    chain: foundry,
    abi: artifact.abi,
    bytecode: `0x${ artifact.evm.bytecode.object }`,
  }));
  expect(deployed.contractAddress).toBeTruthy();
  address = deployed.contractAddress!;

  for (const tokenId of [ BigInt(0), BigInt(17) ]) {
    await commit(await walletClient.writeContract({
      account: owner,
      chain: foundry,
      address,
      abi: artifact.abi,
      functionName: 'mint',
      args: [ owner, tokenId ],
    }));
    await commit(await walletClient.writeContract({
      account: owner,
      chain: foundry,
      address,
      abi: NftArtifact.abi,
      functionName: 'approve',
      args: [ spender, tokenId ],
    }));
  }

  await commit(await walletClient.writeContract({
    account: owner,
    chain: foundry,
    address,
    abi: NftArtifact.abi,
    functionName: 'setApprovalForAll',
    args: [ spender, true ],
  }));
}, 30_000);

afterEach(async() => {
  await provider?.disconnect();
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
