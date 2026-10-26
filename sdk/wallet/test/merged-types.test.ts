import ts from 'typescript';
import { expect, expectTypeOf, test } from 'vitest';
import type {
  AssetPage,
  Balances,
  BankAsset,
  BankBalance,
  ChainAccount,
  ChainHistoryItem,
  ChainHistoryPage,
  ConsensusHead,
  Hex,
  HistoryItem,
  JoinedAccount,
  SequencerReceiptAnswer,
  SequencerRung,
  SendTxResponse,
} from '../src/index.js';

// Prepare the public SDK program once; the test checks its type diagnostics.
const packageDir = decodeURIComponent(new URL('..', import.meta.url).pathname);
const configPath = `${packageDir}tsconfig.json`;
const config = ts.readConfigFile(configPath, ts.sys.readFile);
const parsed = ts.parseJsonConfigFileContent(config.config, ts.sys, packageDir, {
  noEmit: true,
  rootDir: packageDir,
});
const testPath = decodeURIComponent(new URL(import.meta.url).pathname);
const program = ts.createProgram([testPath], parsed.options);

function merged_types_type_every_merged_read_and_the_sequencer_receipt() {
  const account: ChainAccount = {
    account: 'pax1account',
    paxeer: { address: '0x1234', balance: '9007199254740993', nonce: '2', height: '77' },
  };
  const balance: BankBalance = { denom: 'apax', amount: account.paxeer.balance };
  const balances: Balances = { account: account.account, height: '77', balances: [balance] };
  const asset: BankAsset = {
    asset_id: 'native:apax',
    denom: 'apax',
    kind: 'native',
    name: 'Paxeer',
    symbol: 'PAX',
    decimals: 18,
    supply: '9007199254740993000000000',
  };
  const assets: AssetPage = { height: '77', assets: [asset], joined_limit: 200 };
  const consensus: ConsensusHead = {
    latest_height: '77',
    latest_block_hash: '0xabcd',
    latest_app_hash: '0x1234',
    catching_up: false,
  };
  const item: ChainHistoryItem = {
    id: 'tx:0',
    height_or_seq: '77',
    kind: 'transfer',
    direction: 'out',
    account: account.account,
    counterparty: 'pax1recipient',
    asset: balance.denom,
    amount: '10',
    tx_id: '0xabcd',
    ordinal: '0',
    final: true,
    decoded: null,
    asset_metadata: null,
  };
  const history: ChainHistoryPage = { account: account.account, items: [item], next_cursor: 'c2' };
  const emptyHistory: ChainHistoryPage = { account: account.account, items: [], next_cursor: null };
  const submitted: SequencerReceiptAnswer = { rung: 'submitted', txHash: '0xabcd', nonce: '0x2' };
  const receipted: SequencerReceiptAnswer = {
    ...submitted,
    rung: 'receipted',
    seq: '0x1',
    height: '0x4d',
    index: '0x0',
    timestamp: '0x1234',
    status: '0x1',
    returnData: '0x',
    logs: [{ address: '0x1234', topics: ['0xabcd'], data: '0x0102', logIndex: '0x0' }],
    gasUsed: '0x5208',
    cumulativeGasUsed: '0x5208',
    effectiveGasPrice: '0x1',
    feeDenom: 'apax',
    feeAmount: '0x5208',
    sponsor: '0x0000000000000000000000000000000000000000',
    from: '0x1234',
    to: '0x5678',
    contractAddress: '0x0000000000000000000000000000000000000000',
    vmError: '',
    keyId: '0xabcd',
    receipt: '0x0102',
  };
  const committed: SequencerReceiptAnswer = {
    ...receipted,
    rung: 'committed',
    blockNumber: '0x4d',
    blockHash: '0x5678',
  };
  const hashOnly: SendTxResponse = { tx_hash: submitted.txHash, address: '0x1234', chain_id: 125 };
  const withReceipt: SendTxResponse = { ...hashOnly, receipt: receipted };

  expectTypeOf<ChainAccount>().toEqualTypeOf<{
    account: string;
    paxeer: { address: string; balance: string; nonce: string; height: string };
  }>();
  expectTypeOf<BankBalance>().toEqualTypeOf<{ denom: string; amount: string }>();
  expectTypeOf<Balances>().toEqualTypeOf<{ account: string; height: string; balances: BankBalance[] }>();
  expectTypeOf<BankAsset>().toEqualTypeOf<{
    asset_id: string;
    denom: string;
    kind: string;
    name: string;
    symbol: string;
    decimals: number;
    supply: string;
  }>();
  expectTypeOf<AssetPage>().toEqualTypeOf<{ height: string; assets: BankAsset[]; joined_limit: number }>();
  expectTypeOf<ConsensusHead>().toEqualTypeOf<{
    latest_height: string;
    latest_block_hash: string;
    latest_app_hash: string;
    catching_up: boolean;
  }>();
  expectTypeOf<ChainHistoryItem>().toEqualTypeOf<Omit<HistoryItem, 'chain' | 'side'>>();
  expectTypeOf<ChainHistoryPage>().toEqualTypeOf<{
    account: string;
    items: ChainHistoryItem[];
    next_cursor: string | null;
  }>();
  expectTypeOf<SequencerRung>().toEqualTypeOf<'submitted' | 'receipted' | 'committed'>();
  expectTypeOf<SequencerReceiptAnswer>().toEqualTypeOf<{
    rung: SequencerRung;
    txHash: Hex;
    nonce: Hex;
    blockNumber?: Hex;
    blockHash?: Hex;
    seq?: Hex;
    height?: Hex;
    index?: Hex;
    timestamp?: Hex;
    status?: Hex;
    returnData?: Hex;
    logs?: { address: Hex; topics: Hex[]; data: Hex; logIndex: Hex }[];
    gasUsed?: Hex;
    cumulativeGasUsed?: Hex;
    effectiveGasPrice?: Hex;
    feeDenom?: string;
    feeAmount?: Hex;
    sponsor?: Hex;
    from?: Hex;
    to?: Hex;
    contractAddress?: Hex;
    vmError?: string;
    keyId?: Hex;
    receipt?: Hex;
  }>();
  expectTypeOf<SendTxResponse['receipt']>().toEqualTypeOf<SequencerReceiptAnswer | undefined>();
  expectTypeOf<SendTxResponse['tx_hash']>().toEqualTypeOf<Hex>();
  expectTypeOf<JoinedAccount['account']>().not.toEqualTypeOf<string>();

  expect(balances).toEqual({
    account: 'pax1account',
    height: '77',
    balances: [{ denom: 'apax', amount: '9007199254740993' }],
  });
  expect(assets.assets).toEqual([asset]);
  expect(assets.joined_limit).toBe(200);
  expect(consensus.latest_height).toBe(balances.height);
  expect(consensus.catching_up).toBe(false);
  expect(history.items).toEqual([item]);
  expect(history.items[0]).not.toHaveProperty('chain');
  expect(history.items[0]).not.toHaveProperty('side');
  expect(history.next_cursor).toBe('c2');
  expect(emptyHistory.next_cursor).toBeNull();
  expect([submitted, receipted, committed].map((answer) => answer.rung)).toEqual([
    'submitted',
    'receipted',
    'committed',
  ]);
  expect(submitted).not.toHaveProperty('seq');
  expect(receipted).not.toHaveProperty('blockNumber');
  expect(committed.blockNumber).toBe('0x4d');
  expect(committed.logs).toEqual(receipted.logs);
  expect(committed.receipt).toBe('0x0102');
  expect(hashOnly.tx_hash).toBe(withReceipt.tx_hash);
  expect(hashOnly.receipt).toBeUndefined();
  expect(withReceipt.receipt).toEqual(receipted);

  // Vitest erases types; compile this test so its type assertions execute as a gate.
  expect(config.error).toBeUndefined();
  const testSource = program.getSourceFile(testPath);
  expect(testSource).toBeDefined();
  const diagnostics = [...parsed.errors, ...ts.getPreEmitDiagnostics(program, testSource)];
  expect(
    diagnostics.map((diagnostic) => ts.flattenDiagnosticMessageText(diagnostic.messageText, '\n')),
  ).toEqual([]);
}

test(
  merged_types_type_every_merged_read_and_the_sequencer_receipt,
  merged_types_type_every_merged_read_and_the_sequencer_receipt,
);
