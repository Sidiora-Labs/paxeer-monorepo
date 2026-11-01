import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { getAbiItem, toEventSelector } from 'viem';

import { expect, test } from 'vitest';

import { paxeerXVaultAbi } from './contracts';

function vaultEventTopicsMatchTheRelayerParityConstants() {
  const paritySource = readFileSync(
    resolve(__dirname, '../../../../../contracts/bridge/evm/test/RelayerAbiParity.t.sol'),
    'utf8',
  );

  for (const [ eventName, constantName ] of [
    [ 'BridgeDeposit', 'BRIDGE_DEPOSIT_TOPIC' ],
    [ 'BridgeRelease', 'BRIDGE_RELEASE_TOPIC' ],
  ] as const) {
    const declaration = paritySource.match(new RegExp(
      `\\bbytes32\\s+internal\\s+constant\\s+${ constantName }\\s*=\\s*(0x[0-9a-fA-F]{64})\\s*;`,
    ));

    expect(declaration, constantName).not.toBeNull();

    const event = getAbiItem({ abi: paxeerXVaultAbi, name: eventName });
    expect(toEventSelector(event), eventName).toBe(declaration![1]);
  }
}

test(vaultEventTopicsMatchTheRelayerParityConstants);
