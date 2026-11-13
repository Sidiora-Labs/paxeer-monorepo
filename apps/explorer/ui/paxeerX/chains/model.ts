import * as v from 'valibot';

const unsignedInteger = v.pipe(v.number(), v.integer(), v.minValue(0), v.maxValue(Number.MAX_SAFE_INTEGER));
const chainSchema = v.object({
  id: v.pipe(unsignedInteger, v.minValue(1), v.maxValue(0xFFFFFFFF)),
  name: v.pipe(v.string(), v.regex(/^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$/)),
  operator: v.pipe(v.string(), v.regex(/^0x[\da-fA-F]{40}$/)),
  status: v.picklist([ 'active', 'paused', 'governance_paused', 'retired' ]),
  chain_id: unsignedInteger,
  created_height: unsignedInteger,
  bond: v.pipe(v.string(), v.regex(/^(0|[1-9]\d*)$/)),
  gas_allowance: unsignedInteger,
  byte_allowance: unsignedInteger,
  fee_share_bps: v.pipe(unsignedInteger, v.maxValue(10000)),
});

export type ChainRecord = v.InferOutput<typeof chainSchema>;
export type ChainListRow = ChainRecord & { launchSymbol: string | null };

export function chainListRow(chain: ChainRecord, launchSymbol: string | null): ChainListRow {
  return { ...chain, launchSymbol };
}

export function parseChainsResponse(json: unknown): Array<ChainRecord> {
  const { chains } = v.parse(v.object({ chains: v.array(chainSchema) }), json);
  const ids = new Set<number>();
  const names = new Set<string>();

  for (const chain of chains) {
    if (chain.chain_id !== 125000000 + chain.id || ids.has(chain.id) || names.has(chain.name)) {
      throw new Error('Invalid chain identity in /v1/chains response');
    }
    ids.add(chain.id);
    names.add(chain.name);
  }

  return chains.sort((a, b) => a.id - b.id);
}

export function statusLabel(status: ChainRecord['status']): string {
  const labels: Record<ChainRecord['status'], string> = {
    active: 'Active',
    paused: 'Paused',
    governance_paused: 'Paused by governance',
    retired: 'Retired',
  };

  return labels[status];
}

export function rpcUrl(base: string, id: number): string {
  if (!Number.isInteger(id) || id < 1 || id > 0xFFFFFFFF) {
    throw new Error('Invalid virtual chain id');
  }
  const url = new URL(base);
  if (![ 'http:', 'https:', 'ws:', 'wss:' ].includes(url.protocol) || url.search || url.hash) {
    throw new Error('Invalid RPC base URL');
  }

  return `${ base.replace(/\/+$/, '') }/chain/${ id }`;
}

export function sortChains(chains: ReadonlyArray<ChainRecord>): Array<ChainRecord> {
  return [ ...chains ].sort((a, b) => Number(b.status === 'active') - Number(a.status === 'active') || a.id - b.id);
}
