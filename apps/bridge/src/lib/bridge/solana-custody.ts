import { hexToBytes } from 'viem';
import { base58Encode, bytesToHex, solanaProgramAddress, solanaRecipientHandle } from './solana';
import type { BridgeAsset } from './chains';
import type { VaultState } from './use-bridge-state';

const encoder = new TextEncoder();
const CONFIG_BYTES = 1367;

type SolanaAccount = { owner: string; executable: boolean; data: [string, 'base64'] };

function accountBytes(value: unknown, programId: string, magic: string, length: number): Uint8Array {
  if (typeof value !== 'object' || value === null) throw new Error('The Solana custody account is missing.');
  const account = value as Partial<SolanaAccount>;
  if (account.owner !== programId || account.executable !== false || !Array.isArray(account.data) ||
    typeof account.data[0] !== 'string' || account.data[1] !== 'base64') {
    throw new Error('The Solana custody account has the wrong owner or encoding.');
  }
  const bytes = Uint8Array.from(atob(account.data[0]), (char) => char.charCodeAt(0));
  if (bytes.length !== length || !encoder.encode(magic).every((byte, i) => bytes[i] === byte) ||
    new DataView(bytes.buffer).getUint16(8, false) !== 1) {
    throw new Error('The Solana custody account has an invalid layout.');
  }
  return bytes;
}

export type SolanaCustodyConfig = { programId: string | undefined; rpcUrl: string | undefined };

export async function readSolanaCustodyState(
  { assetId, decimals, solanaMint }: Pick<BridgeAsset, 'assetId' | 'decimals' | 'solanaMint'>,
  { programId, rpcUrl }: SolanaCustodyConfig,
  signal?: AbortSignal,
): Promise<VaultState> {
  const configPda = programId && solanaProgramAddress(programId, [ encoder.encode('config') ]);
  const vaultPda = programId && solanaProgramAddress(programId, [ encoder.encode('vault-authority') ]);
  if (!programId || !rpcUrl || !configPda || !vaultPda || !solanaMint) throw new Error('The Solana custody program is not configured.');
  const rpc = async(method: string, params: unknown[]) => {
    const response = await fetch(rpcUrl, {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, cache: 'no-store', signal,
      body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }),
    });
    if (!response.ok) throw new Error(`The Solana RPC answered ${ response.status }.`);
    const message: unknown = await response.json();
    if (typeof message !== 'object' || message === null || !('jsonrpc' in message) || message.jsonrpc !== '2.0' ||
      !('id' in message) || message.id !== 1 || 'error' in message || !('result' in message)) {
      throw new Error('The Solana RPC returned an invalid custody response.');
    }
    return message.result;
  };
  const [ configResult, assets ] = await Promise.all([
    rpc('getAccountInfo', [ configPda.address, { encoding: 'base64', commitment: 'finalized' } ]),
    rpc('getProgramAccounts', [ programId, {
      encoding: 'base64', commitment: 'finalized',
      filters: [ { dataSize: 89 }, { memcmp: { offset: 42, bytes: base58Encode(hexToBytes(assetId)) } } ],
    } ]),
  ]);
  if (typeof configResult !== 'object' || configResult === null || !('value' in configResult)) {
    throw new Error('The Solana RPC returned an invalid custody config result.');
  }
  const config = accountBytes(configResult.value, programId, 'PXBRCFG0', CONFIG_BYTES);
  if (config[74] > 1 || config[75] > 64 || config[76] > config[75] || config[76] === 0 ||
    config[77] !== configPda.bump || config[78] !== vaultPda.bump) {
    throw new Error('The Solana custody config has invalid flags, threshold or PDA bumps.');
  }
  if (!Array.isArray(assets) || assets.length !== 1 || typeof assets[0] !== 'object' || assets[0] === null ||
    !('account' in assets[0]) || !('pubkey' in assets[0])) {
    throw new Error('The Solana custody asset is not uniquely registered.');
  }
  const asset = accountBytes(assets[0].account, programId, 'PXBRAST0', 89);
  const mint = asset.slice(10, 42);
  const assetPda = solanaProgramAddress(programId, [ encoder.encode('asset'), mint ]);
  if (!assetPda || assets[0].pubkey !== assetPda.address || asset[64] !== assetPda.bump ||
    bytesToHex(asset.subarray(42, 62)).toLowerCase() !== assetId.toLowerCase() ||
    base58Encode(mint) !== solanaMint ||
    asset[62] !== decimals || asset[63] > 1) {
    throw new Error('The Solana custody asset does not match its mint, PDA or configured identity.');
  }
  const view = new DataView(asset.buffer);
  return {
    paused: config[74] === 1,
    enabled: asset[63] === 1,
    vault: solanaRecipientHandle(vaultPda.address),
    perTx: view.getBigUint64(65, false),
    total: view.getBigUint64(73, false),
    outstanding: view.getBigUint64(81, false),
    attestors: Array.from({ length: config[75] }, (_, index) => bytesToHex(config.subarray(87 + index * 20, 107 + index * 20))),
    threshold: config[76],
    loaded: true,
    error: false,
  };
}
