import {
  getAddress,
  keccak256,
  type Address,
  type Hash,
  type PublicClient,
} from 'viem';

export type RolePin = {
  address: Address;
  runtimeHash: Hash;
} & (
  | { kind: 'surface'; implementation?: never; implementationHash?: never }
  | { kind: 'erc1967'; implementation: Address; implementationHash: Hash }
);

const implementationSlot =
  '0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc';

export async function verifyRolePins(
  reader: Pick<PublicClient, 'getCode' | 'getStorageAt'>,
  pins: readonly RolePin[],
  block: bigint,
): Promise<void> {
  for (const pin of pins) {
    const code = await reader.getCode({
      address: pin.address,
      blockNumber: block,
    });
    if (!code || code === '0x' || keccak256(code) !== pin.runtimeHash) {
      throw new Error('Native trading code identity changed');
    }
    if (pin.kind === 'erc1967') {
      const [storage, implementation] = await Promise.all([
        reader.getStorageAt({
          address: pin.address,
          slot: implementationSlot,
          blockNumber: block,
        }),
        reader.getCode({ address: pin.implementation, blockNumber: block }),
      ]);
      if (
        !storage ||
        getAddress(`0x${storage.slice(-40)}`) !==
          getAddress(pin.implementation) ||
        !implementation ||
        implementation === '0x' ||
        keccak256(implementation) !== pin.implementationHash
      ) {
        throw new Error('Native trading code identity changed');
      }
    }
  }
}
