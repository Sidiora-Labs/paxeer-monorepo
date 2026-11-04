export type ApiName =
'general' | 'admin' | 'bens' | 'contractInfo' | 'clusters' | 'external' | 'interchainIndexer' |
'metadata' | 'multichainAggregator' | 'multichainStats' | 'paxeerXIndexer' | 'rewards' | 'stats' | 'tac' |
'userOps' | 'visualize' | 'zetachain';

export interface ApiResource {
  api?: ApiName;
  path: string;
  pathParams?: Array<string>;
  filterFields?: Array<string>;
  paginated?: boolean;
  headers?: RequestInit['headers'];
}
