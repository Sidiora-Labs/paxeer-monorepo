// @vitest-environment jsdom
import { afterEach, beforeEach, expect, test } from 'vitest';
import { ethers } from 'ethers';
import { IdentitySession } from './identity';
import { embeddedWallet } from './session';
import { WalletSigner, WalletSignerError } from './signer';
import { runProviderSwitchRestartCoverage } from './test/connected-network';
import {
  EMAIL,
  EMAIL_CODE,
  fixtureBody,
  loadConstructions,
  startGateway,
  type TestGateway,
} from './test/gateway';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let gateway: TestGateway;
let identity: IdentitySession;
let readProvider: ethers.JsonRpcProvider;
let signer: WalletSigner;

beforeEach(async () => {
  localStorage.clear();
  gateway = await startGateway();
  identity = IdentitySession.create(gateway.config, {
    persistSession: false,
    detectSessionInUrl: false,
  });
  await identity.verifyEmailCode(EMAIL, EMAIL_CODE);
  readProvider = new ethers.JsonRpcProvider(gateway.config.rpcUrl, 125, {
    staticNetwork: true,
    batchMaxCount: 1,
  });
  signer = new WalletSigner(embeddedWallet(gateway.config, identity), readProvider);
});

afterEach(async () => {
  signer.wallet.dispose();
  readProvider.destroy();
  await identity.client.auth.stopAutoRefresh();
  await gateway.close();
  localStorage.clear();
});

async function provider_switch_restart_coverage() {
  await runProviderSwitchRestartCoverage();
}

type SupportedField =
  | 'from'
  | 'to'
  | 'data'
  | 'value'
  | 'gasLimit'
  | 'maxFeePerGas'
  | 'maxPriorityFeePerGas'
  | 'nonce'
  | 'chainId';

async function signer_extra_fields_discarded_rejects_unsupported_input() {
  const recipient = loadConstructions().transaction.to;
  const unsupported: Record<Exclude<keyof ethers.TransactionRequest, SupportedField>, unknown> = {
    gasPrice: 0n,
    type: 0,
    accessList: [{ address: recipient, storageKeys: [ethers.ZeroHash] }],
    authorizationList: [
      await new ethers.Wallet(
        ethers.keccak256(ethers.toUtf8Bytes('authorization fixture'))
      ).authorize({ address: recipient, chainId: 125, nonce: 0 }),
    ],
    customData: { feeCurrency: recipient },
    blockTag: 'pending',
    enableCcipRead: false,
    blobVersionedHashes: [`0x01${'00'.repeat(31)}`],
    maxFeePerBlobGas: 0n,
    blobs: [new Uint8Array(131072)],
    kzg: {},
    blobWrapperVersion: 0,
  };
  for (const [field, value] of [
    ...Object.entries(unsupported),
    ['accessList', []],
    ['authorizationList', []],
    ['unexpectedField', 1],
  ] as const) {
    const tx: ethers.TransactionRequest = { to: recipient, value: 1n };
    Reflect.set(tx, field, value);
    await expect(signer.sendTransaction(tx), field).rejects.toMatchObject({
      name: 'WalletSignerError',
      code: 'invalid_field',
      message: `unsupported transaction field: ${field}`,
    });
    expect(
      gateway.requests.filter((entry) => entry.path === '/v1/wallet/send'),
      field
    ).toEqual([]);
  }
  const inherited: ethers.TransactionRequest = Object.create({ gasPrice: 1n });
  inherited.to = recipient;
  const hidden: ethers.TransactionRequest = { to: recipient };
  Object.defineProperty(hidden, 'type', { value: 2 });
  const symbol = Symbol('transaction extension');
  const extended: ethers.TransactionRequest = { to: recipient };
  Reflect.set(extended, symbol, 'unsupported');
  for (const tx of [inherited, hidden, extended]) {
    await expect(signer.sendTransaction(tx)).rejects.toBeInstanceOf(WalletSignerError);
    expect(gateway.requests.filter((entry) => entry.path === '/v1/wallet/send')).toEqual([]);
  }
}

async function signer_extra_fields_discarded_preserves_supported_fields() {
  const constructions = loadConstructions();
  const tx = constructions.transaction;
  const response = await signer.sendTransaction({
    from: Promise.resolve(constructions.address),
    to: Promise.resolve(tx.to),
    data: tx.data,
    value: BigInt(tx.value),
    gasLimit: BigInt(tx.gas),
    maxFeePerGas: BigInt(tx.maxFeePerGas),
    maxPriorityFeePerGas: BigInt(tx.maxPriorityFeePerGas),
    nonce: tx.nonce,
    chainId: tx.chainId,
    gasPrice: null,
    accessList: undefined,
  });
  expect(
    gateway.requests.filter((entry) => entry.path === '/v1/wallet/send').map((entry) => entry.body)
  ).toEqual([
    {
      tx: {
        from: constructions.address,
        to: ethers.getAddress(tx.to),
        data: tx.data,
        value: tx.value,
        gas: tx.gas,
        maxFeePerGas: tx.maxFeePerGas,
        maxPriorityFeePerGas: tx.maxPriorityFeePerGas,
        nonce: tx.nonce,
        chainId: tx.chainId,
      },
    },
  ]);
  expect(response.hash).toBe(fixtureBody('default', 'POST', '/v1/wallet/send').tx_hash);
  expect(response.from).toBe(constructions.address);
  expect(response.to).toBe(ethers.getAddress(tx.to));
  expect(response.data).toBe(tx.data);
  expect(response.value).toBe(BigInt(tx.value));
  expect(response.gasLimit).toBe(BigInt(tx.gas));
  expect(response.maxFeePerGas).toBe(BigInt(tx.maxFeePerGas));
  expect(response.maxPriorityFeePerGas).toBe(BigInt(tx.maxPriorityFeePerGas));
  expect(response.nonce).toBe(tx.nonce);
  expect(response.chainId).toBe(BigInt(tx.chainId));
  expect(response.type).toBe(2);
}

test(provider_switch_restart_coverage.name, provider_switch_restart_coverage);
test(
  signer_extra_fields_discarded_rejects_unsupported_input.name,
  signer_extra_fields_discarded_rejects_unsupported_input
);
test(
  signer_extra_fields_discarded_preserves_supported_fields.name,
  signer_extra_fields_discarded_preserves_supported_fields
);
