import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { toFunctionSignature as toSignature } from 'viem';
import { formatAbiItem } from 'viem/utils';

import { expect, test } from 'vitest';

import * as contracts from './contracts';

const layerxBridgeAbi = contracts.layerxBridgeAbi;
const paxeerXVaultAbi = contracts.paxeerXVaultAbi;
const repositoryRoot = resolve(__dirname, '../../../../..');

function parameterType(parameter: string): string {
  const type = parameter.trim().split(/\s+/)[0];
  return type.replace(/^(u?int)(?=\[|$)/, '$1' + '256');
}

function vaultSourceSignatures(source: string): Set<string> {
  const signatures = new Set<string>();
  const declarations = /\b(function|event|error)\s+(\w+)\s*\(([^()]*)\)([^;{]*)[;{]/g;

  for (const [ , kind, name, parameters, modifiers ] of source.matchAll(declarations)) {
    if (kind === 'function' && !/\b(public|external)\b/.test(modifiers)) {
      continue;
    }
    const types = parameters.trim() === '' ? [] : parameters.split(',').map(parameterType);
    signatures.add(`${ kind }:${ name }(${ types.join(',') })`);
  }

  for (const [ , keyType, name ] of source.matchAll(/\bmapping\s*\(\s*(\w+)\s*=>[^;]+\)\s+public\s+(\w+)\s*;/g)) {
    signatures.add(`function:${ name }(${ parameterType(keyType) })`);
  }
  for (const [ , name ] of source.matchAll(/\b(?:bool|address|bytes\d*|u?int\d*)\s+public\s+(\w+)\s*;/g)) {
    signatures.add(`function:${ name }()`);
  }

  return signatures;
}

function layerxBridgeAbiEqualsThePrecompileAbi() {
  const precompileAbi = JSON.parse(readFileSync(resolve(repositoryRoot, 'precompiles/layerxbridge/abi.json'), 'utf8'));

  expect(layerxBridgeAbi).toEqual(precompileAbi);
}

function paxeerXVaultAbiSelectorsExistInTheVaultSource() {
  const source = readFileSync(resolve(repositoryRoot, 'contracts/bridge/evm/src/PaxeerXVault.sol'), 'utf8')
    .replace(/\/\*[\s\S]*?\*\/|\/\/[^\n]*/g, '');
  const signatures = vaultSourceSignatures(source);

  expect(source).toMatch(/import\s*\{\s*Ownable2Step\s*\}\s*from\s*["']@openzeppelin\/contracts\/access\/Ownable2Step\.sol["']\s*;/);
  expect(source).toMatch(/\bcontract\s+PaxeerXVault\s+is\s+Ownable2Step\s*(?:,|\{)/);

  expect(paxeerXVaultAbi.length).toBeGreaterThan(0);
  for (const item of paxeerXVaultAbi) {
    const signature = toSignature(formatAbiItem(item));
    if (item.type === 'function' && item.name === 'owner') {
      expect(signature).toBe('owner()');
    } else {
      expect(signatures, `${ item.type } ${ signature } must be declared in PaxeerXVault.sol`)
        .toContain(`${ item.type }:${ signature }`);
    }
  }
}

test(layerxBridgeAbiEqualsThePrecompileAbi.name, layerxBridgeAbiEqualsThePrecompileAbi);
test(paxeerXVaultAbiSelectorsExistInTheVaultSource.name, paxeerXVaultAbiSelectorsExistInTheVaultSource);
