import { afterEach, expect, test, vi } from 'vitest';
import { ethers } from 'ethers';
import {
  checkAvailability,
  commitName,
  controllerAbi,
  getMinCommitmentAge,
  getRentPrice,
  getWebsite,
  PAXEER_CONTENTHASH_TAG,
  pnsConfigured,
  PnsNotConfiguredError,
  readPnsContracts,
  renewName,
  resolverAbi,
  setPrimaryName,
  setWebsite,
  transferName,
  websiteUrl,
} from './pns';

const resolverContract = vi.hoisted(() => ({
  setContenthash: vi.fn(),
  contenthash: vi.fn(),
}));

vi.mock('ethers', async (importOriginal) => {
  const actual = await importOriginal<typeof import('ethers')>();
  return {
    ...actual,
    ethers: {
      ...actual.ethers,
      Contract: vi.fn(function () {
        return resolverContract;
      }),
      JsonRpcProvider: vi.fn(),
    },
  };
});

afterEach(() => {
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
  vi.clearAllMocks();
});

const contractVariables = [
  'NEXT_PUBLIC_PNS_CONTROLLER',
  'NEXT_PUBLIC_PNS_PUBLIC_RESOLVER',
  'NEXT_PUBLIC_PNS_REVERSE_REGISTRAR',
] as const;

function configureContracts() {
  vi.stubEnv('NEXT_PUBLIC_PNS_CONTROLLER', `0x${'11'.repeat(20)}`);
  vi.stubEnv('NEXT_PUBLIC_PNS_PUBLIC_RESOLVER', `0x${'22'.repeat(20)}`);
  vi.stubEnv('NEXT_PUBLIC_PNS_REVERSE_REGISTRAR', `0x${'33'.repeat(20)}`);
}

async function pns_refuses_writes_until_the_contracts_are_configured() {
  for (const variable of contractVariables) vi.stubEnv(variable, undefined);
  const signer = new ethers.Wallet(`0x${'01'.repeat(32)}`);
  const getAddress = vi.spyOn(signer, 'getAddress');
  const sendTransaction = vi.spyOn(signer, 'sendTransaction');
  const params = { label: 'alice', owner: signer.address, durationSeconds: 365 * 24 * 60 * 60 };

  expect(pnsConfigured()).toBe(false);
  expect(() => readPnsContracts()).toThrow('name service not configured');
  await expect(commitName(signer, params)).rejects.toBeInstanceOf(PnsNotConfiguredError);
  await expect(renewName(signer, 'alice', params.durationSeconds)).rejects.toBeInstanceOf(
    PnsNotConfiguredError
  );
  await expect(
    transferName(signer, { id: 'alice', name: 'alice.pax' }, signer.address)
  ).rejects.toBeInstanceOf(PnsNotConfiguredError);
  await expect(setPrimaryName(signer, 'alice.pax')).rejects.toBeInstanceOf(PnsNotConfiguredError);
  await expect(setWebsite(signer, 'alice', `0x${'ab'.repeat(32)}`)).rejects.toBeInstanceOf(
    PnsNotConfiguredError
  );
  await expect(checkAvailability('alice')).rejects.toBeInstanceOf(PnsNotConfiguredError);
  await expect(getRentPrice('alice', params.durationSeconds)).rejects.toBeInstanceOf(
    PnsNotConfiguredError
  );
  await expect(getMinCommitmentAge()).rejects.toBeInstanceOf(PnsNotConfiguredError);
  await expect(getWebsite('alice')).rejects.toBeInstanceOf(PnsNotConfiguredError);

  for (const variable of contractVariables) {
    configureContracts();
    vi.stubEnv(variable, undefined);
    expect(pnsConfigured()).toBe(false);
    await expect(commitName(signer, params)).rejects.toBeInstanceOf(PnsNotConfiguredError);
    for (const invalid of ['', '0x1234', `0x${'11'.repeat(21)}`, `0x${'gg'.repeat(20)}`]) {
      vi.stubEnv(variable, invalid);
      expect(pnsConfigured()).toBe(false);
      await expect(commitName(signer, params)).rejects.toBeInstanceOf(PnsNotConfiguredError);
    }
  }
  expect(ethers.JsonRpcProvider).not.toHaveBeenCalled();
  expect(ethers.Contract).not.toHaveBeenCalled();
  expect(getAddress).not.toHaveBeenCalled();
  expect(sendTransaction).not.toHaveBeenCalled();

  configureContracts();
  expect(pnsConfigured()).toBe(true);
  expect(readPnsContracts()).toEqual({
    controller: `0x${'11'.repeat(20)}`,
    publicResolver: `0x${'22'.repeat(20)}`,
    reverseRegistrar: `0x${'33'.repeat(20)}`,
  });
}

async function pns_website_helpers_encode_the_paxeer_tag() {
  configureContracts();
  const signer = new ethers.Wallet(`0x${'01'.repeat(32)}`);
  const digest = `0x${'ab'.repeat(32)}`;
  const encoded = `0x70617831${digest.slice(2)}`;
  const node = ethers.namehash('alice.pax');
  const wait = vi.fn().mockResolvedValue({ status: 1 });
  resolverContract.setContenthash.mockResolvedValue({ hash: ethers.ZeroHash, wait });

  expect(PAXEER_CONTENTHASH_TAG).toBe(0x70617831);
  await expect(setWebsite(signer, 'alice', digest)).resolves.toBe(ethers.ZeroHash);
  expect(ethers.Contract).toHaveBeenCalledWith(
    readPnsContracts().publicResolver,
    resolverAbi,
    signer
  );
  expect(resolverContract.setContenthash).toHaveBeenCalledExactlyOnceWith(node, encoded);
  expect(wait).toHaveBeenCalledExactlyOnceWith();

  resolverContract.contenthash.mockResolvedValueOnce(encoded);
  await expect(getWebsite('alice')).resolves.toEqual({ digest });
  expect(resolverContract.contenthash).toHaveBeenCalledExactlyOnceWith(node);
  resolverContract.contenthash.mockResolvedValueOnce(`0x70617831${'ab'.repeat(31)}`);
  await expect(getWebsite('alice')).resolves.toBeNull();
  resolverContract.contenthash.mockResolvedValueOnce(`0x70617832${digest.slice(2)}`);
  await expect(getWebsite('alice')).resolves.toBeNull();
  resolverContract.contenthash.mockResolvedValueOnce('0x');
  await expect(getWebsite('alice')).resolves.toBeNull();
  await expect(setWebsite(signer, 'alice', `0x${'ab'.repeat(31)}`)).rejects.toThrow(TypeError);
  await expect(setWebsite(signer, 'alice', digest.slice(2))).rejects.toThrow(TypeError);
  expect(resolverContract.setContenthash).toHaveBeenCalledTimes(1);

  vi.stubEnv('NEXT_PUBLIC_PNS_SHELL_DOMAIN', undefined);
  expect(websiteUrl('alice')).toBeNull();
  vi.stubEnv('NEXT_PUBLIC_PNS_SHELL_DOMAIN', 'sites.example');
  expect(websiteUrl('alice')).toBe('https://alice.pax.sites.example/');
}

function pns_controller_abi_keeps_the_registration_tuple_order() {
  const iface = new ethers.Interface(controllerAbi);
  for (const method of ['makeCommitment', 'register']) {
    const registration = iface.getFunction(method)!.inputs[0];
    expect(registration.type).toBe('tuple');
    expect(registration.components!.map((component) => component.name)).toEqual([
      'label',
      'owner',
      'duration',
      'secret',
      'resolver',
      'data',
      'reverseRecord',
      'referrer',
    ]);
    expect(registration.components!.map((component) => component.type)).toEqual([
      'string',
      'address',
      'uint256',
      'bytes32',
      'address',
      'bytes[]',
      'uint8',
      'bytes32',
    ]);
  }
}

test(pns_refuses_writes_until_the_contracts_are_configured);
test(pns_website_helpers_encode_the_paxeer_tag);
test(pns_controller_abi_keeps_the_registration_tuple_order);
