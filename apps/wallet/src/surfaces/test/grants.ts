import { ADDR_PRECOMPILE_ADDRESS, abiSelector } from '@sidiora/layerx-sdk';
import { LEDGER_PRECOMPILE, PAYER_GRANT_BYTES, ledgerLogFloor } from '@paxeer/wallet';
import {
  encodeAbiParameters,
  encodePacked,
  keccak256,
  parseAbiParameters,
  size,
  toHex,
  type Hex,
} from 'viem';
import { SurfaceChain } from './chain';

export const DID_PUBLIC_KEY: Hex = `0x${'5d'.repeat(32)}`;
export const DID = `did:layerx:${'5d'.repeat(32)}`;
export const ACCOUNT_ID: Hex = `0x${'c3'.repeat(32)}`;
export const FOREIGN_ACCOUNT_ID: Hex = `0x${'c4'.repeat(32)}`;
export const FLOOR = ledgerLogFloor(125)!;
export const FOREIGN_ADDRESS = `0x${'f1'.repeat(20)}`;
export const SPONSOR_ADDRESS = `0x${'5e'.repeat(20)}`;
export const HEAD = { number: toHex(FLOOR + 42n), stateRoot: `0x${'e5'.repeat(32)}` };
export const GET_LAYERX_DID = abiSelector('getLayerXDid(address)');
export const ACCOUNT_ID_SELECTOR = abiSelector('accountId(bytes32,bytes32)');
export const GET_PAYER_GRANT = abiSelector('getPayerGrant(bytes32)');
export const PAYER_GRANT_ISSUED = keccak256(toHex('PayerGrantIssued(bytes32)'));

export interface IssuedGrant {
  readonly grantId: Hex;
  readonly from: Hex;
  readonly drawnThisPeriod: bigint;
  readonly block: bigint;
  readonly expiration?: bigint;
  readonly sponsored?: boolean;
}

export const issuingTx = (grant: IssuedGrant): Hex => keccak256(grant.grantId);

export const OWNED_GRANT: IssuedGrant = {
  grantId: `0x${'a1'.repeat(32)}`,
  from: ACCOUNT_ID,
  drawnThisPeriod: 250n,
  block: FLOOR + 1n,
};
export const FOREIGN_GRANT: IssuedGrant = {
  grantId: `0x${'a2'.repeat(32)}`,
  from: FOREIGN_ACCOUNT_ID,
  drawnThisPeriod: 0n,
  block: FLOOR + 1n,
};

function payerGrantAnswer(grant: IssuedGrant): Hex {
  const wire = encodePacked(
    [
      'bytes32',
      'bytes32',
      'bytes32',
      'bytes32',
      'uint128',
      'uint128',
      'bool',
      'uint64',
      'uint64',
      'bytes32',
      'bool',
      'bytes32',
      'uint64',
      'bytes32',
      'bytes',
    ],
    [
      grant.grantId,
      grant.from,
      `0x${'f6'.repeat(32)}`,
      `0x${'d4'.repeat(32)}`,
      1000n,
      50000n,
      true,
      86400n,
      grant.expiration ?? 1_900_000_000n,
      `0x${'77'.repeat(32)}`,
      false,
      `0x${'00'.repeat(32)}`,
      0n,
      DID_PUBLIC_KEY,
      `0x${'99'.repeat(64)}`,
    ]
  );
  if (size(wire) !== PAYER_GRANT_BYTES) throw new Error(`payer grant wire is ${size(wire)} bytes`);
  return encodeAbiParameters(
    parseAbiParameters('bytes, uint128, uint128, uint64, uint64, bool, bool'),
    [wire, 1250n, grant.drawnThisPeriod, 1_800_000_000n, 0n, false, false]
  );
}

export class GrantChain extends SurfaceChain {
  didPublicKey: Hex = DID_PUBLIC_KEY;
  finalised: typeof HEAD | null = HEAD;
  issued: readonly IssuedGrant[] = [];
  readonly logQueries: { fromBlock: string; toBlock: string }[] = [];
  readonly blockQueries: unknown[] = [];
  readonly callBlocks: unknown[] = [];
  readonly grantCallBlocks: unknown[] = [];
  failLogsFrom: bigint | null = null;
  capsChainId = 125;
  readonly txQueries: string[] = [];
  inFlight = 0;
  peakInFlight = 0;
  holderRecords: ((params: readonly unknown[]) => unknown) | null = null;
  readonly holderQueries: (readonly unknown[])[] = [];

  override async request(args: {
    method: string;
    params?: readonly unknown[] | object;
  }): Promise<unknown> {
    const params = Array.isArray(args.params) ? (args.params as readonly unknown[]) : [];
    if (args.method === 'eth_getBlockByNumber') {
      this.methods.push(args.method);
      this.blockQueries.push(params[0]);
      if (params[0] !== 'finalized') throw new Error('only the finalised block is served');
      return this.finalised === null ? null : { ...this.finalised, hash: `0x${'b7'.repeat(32)}` };
    }
    if (args.method === 'eth_chainId') return toHex(this.capsChainId);
    if (args.method === 'px_listHolderRecords') {
      this.methods.push(args.method);
      this.holderQueries.push(params);
      if (this.holderRecords === null)
        throw Object.assign(new Error('method not found'), { code: -32601 });
      return this.holderRecords(params);
    }
    if (args.method === 'eth_getTransactionByHash') {
      this.methods.push(args.method);
      this.txQueries.push(params[0] as string);
      const grant = this.issued.find((candidate) => issuingTx(candidate) === params[0]);
      if (!grant) return null;
      const own = grant.from === ACCOUNT_ID;
      if (grant.sponsored)
        return { hash: params[0], from: SPONSOR_ADDRESS, to: own ? this.address : FOREIGN_ADDRESS };
      return { hash: params[0], from: own ? this.address : FOREIGN_ADDRESS, to: LEDGER_PRECOMPILE };
    }
    if (args.method === 'eth_getLogs') {
      this.methods.push(args.method);
      const query = params[0] as { fromBlock: string; toBlock: string };
      this.logQueries.push(query);
      this.inFlight += 1;
      this.peakInFlight = Math.max(this.peakInFlight, this.inFlight);
      await new Promise((resolve) => setTimeout(resolve, 0));
      this.inFlight -= 1;
      if (this.failLogsFrom !== null && BigInt(query.fromBlock) === this.failLogsFrom)
        throw new Error('log range unavailable');
      return this.issued
        .filter(
          (grant) => grant.block >= BigInt(query.fromBlock) && grant.block <= BigInt(query.toBlock)
        )
        .map((grant) => ({
          address: LEDGER_PRECOMPILE,
          topics: [PAYER_GRANT_ISSUED, grant.grantId],
          data: '0x',
          transactionHash: issuingTx(grant),
        }));
    }
    if (args.method === 'eth_call') {
      this.callBlocks.push(params[1]);
      const call = params[0] as { to: string; data: string };
      const to = call.to.toLowerCase();
      const data = call.data.toLowerCase();
      const answer = this.grantView(to, data);
      if (answer !== null) {
        this.grantCallBlocks.push(params[1]);
        this.methods.push(args.method);
        this.calls.push({ to, data });
        return answer;
      }
    }
    return super.request(args);
  }

  private grantView(to: string, data: string): Hex | null {
    if (
      to === ADDR_PRECOMPILE_ADDRESS &&
      data === `${GET_LAYERX_DID}${this.address.slice(2).padStart(64, '0')}`
    ) {
      return encodeAbiParameters(parseAbiParameters('bytes32, string'), [this.didPublicKey, DID]);
    }
    if (
      to === LEDGER_PRECOMPILE &&
      data === `${ACCOUNT_ID_SELECTOR}${this.didPublicKey.slice(2)}${'00'.repeat(32)}`
    ) {
      return encodeAbiParameters(parseAbiParameters('bytes32'), [ACCOUNT_ID]);
    }
    const grant = this.issued.find(
      (candidate) =>
        to === LEDGER_PRECOMPILE && data === `${GET_PAYER_GRANT}${candidate.grantId.slice(2)}`
    );
    return grant ? payerGrantAnswer(grant) : null;
  }
}
