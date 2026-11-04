import ts from 'typescript';
import { committedReceipt, nodeVector } from './support/receipt-recordings.js';
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
  SequencerReceiptAnswer,
  SequencerReceiptView,
  SequencerRung,
  SendTxResponse,
  UnifiedAccountDocument,
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
    account: {
      evm_address: '0x102132435465768798a9bacbdcedfe0f1e2d3c4b',
      pax_address: 'pax1exampleaccount',
      layerx_did: `did:layerx:${'61'.repeat(32)}`,
      layerx_account: '457d8ff18e80b2c0671cb8dd37042120b6bfcfa2f2477389636a6d62e2e64de0',
      bound: true,
    },
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
    account: 'pax1account',
    counterparty: 'pax1recipient',
    asset: balance.denom,
    amount: '10',
    tx_id: '0xabcd',
    ordinal: '0',
    final: true,
    decoded: null,
    asset_metadata: null,
  };
  const history: ChainHistoryPage = { account: 'pax1account', items: [item], next_cursor: 'c2' };
  const emptyHistory: ChainHistoryPage = { account: 'pax1account', items: [], next_cursor: null };
  const receipted = nodeVector('V1-pax-transfer').receiptJson as Extract<
    SequencerReceiptAnswer,
    { rung: 'receipted' }
  >;
  const submitted: SequencerReceiptAnswer = {
    rung: 'submitted',
    txHash: receipted.txHash,
    nonce: receipted.nonce,
  };
  const committed = committedReceipt as Extract<SequencerReceiptAnswer, { rung: 'committed' }>;
  const hashOnly: SendTxResponse = { tx_hash: submitted.txHash, address: '0x1234', chain_id: 125 };
  const withReceipt: SendTxResponse = { ...hashOnly, receipt: receipted };

  expectTypeOf<ChainAccount>().toEqualTypeOf<{
    account: UnifiedAccountDocument;
    paxeer: { address: string; balance: string; nonce: string; height: string };
  }>();
  expectTypeOf<BankBalance>().toEqualTypeOf<{ denom: string; amount: string }>();
  expectTypeOf<Balances>().toEqualTypeOf<{
    account: UnifiedAccountDocument;
    height: string;
    balances: BankBalance[];
  }>();
  expectTypeOf<BankAsset>().toEqualTypeOf<{
    asset_id: string;
    denom: string | null;
    kind: string;
    name: string | null;
    symbol: string | null;
    decimals: number | null;
    supply: string | null;
  }>();
  expectTypeOf<AssetPage>().toEqualTypeOf<{
    height: string;
    assets: BankAsset[];
    joined_limit: number;
  }>();
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
  expectTypeOf<SequencerReceiptView>().toEqualTypeOf<{
    seq: Hex;
    height: Hex;
    index: Hex;
    timestamp: Hex;
    status: Hex;
    returnData: Hex;
    logs: { address: Hex; topics: Hex[]; data: Hex; logIndex: Hex }[];
    gasUsed: Hex;
    cumulativeGasUsed: Hex;
    effectiveGasPrice: Hex;
    paymentDenom: string;
    feeDenom: string;
    feeAmount: Hex;
    sponsor: Hex;
    from: Hex;
    to: Hex;
    contractAddress: Hex;
    vmError: string;
    keyId: Hex;
    signingKey: Hex;
    receipt: Hex;
  }>();
  expectTypeOf<
    Extract<SequencerReceiptAnswer, { rung: 'submitted' }>['seq']
  >().toEqualTypeOf<undefined>();
  expectTypeOf<
    Extract<SequencerReceiptAnswer, { rung: 'submitted' }>['blockNumber']
  >().toEqualTypeOf<undefined>();
  expectTypeOf<Extract<SequencerReceiptAnswer, { rung: 'receipted' }>>().toEqualTypeOf<
    { txHash: Hex; nonce: Hex } & SequencerReceiptView & {
        rung: 'receipted';
        blockNumber?: never;
        blockHash?: never;
      }
  >();
  expectTypeOf<Extract<SequencerReceiptAnswer, { rung: 'committed' }>>().toEqualTypeOf<
    { txHash: Hex; nonce: Hex } & SequencerReceiptView & {
        rung: 'committed';
        blockNumber: Hex;
        blockHash: Hex;
      }
  >();
  expectTypeOf<SendTxResponse['receipt']>().toEqualTypeOf<SequencerReceiptAnswer | undefined>();
  expectTypeOf<SendTxResponse['tx_hash']>().toEqualTypeOf<Hex>();
  expectTypeOf<ChainAccount['account']>().not.toEqualTypeOf<string>();

  expect(balances).toEqual({
    account: account.account,
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
  expect(committed.blockNumber).toBe(committedReceipt.blockNumber);
  expect(committed.logs).toEqual(committedReceipt.logs);
  expect(committed.receipt).toBe(committedReceipt.receipt);
  expect(hashOnly.tx_hash).toBe(withReceipt.tx_hash);
  expect(hashOnly.receipt).toBeUndefined();
  expect(withReceipt.receipt).toEqual(receipted);

  // Vitest erases types; compile this test so its type assertions execute as a gate.
  expect(config.error).toBeUndefined();
  const testSource = program.getSourceFile(testPath);
  expect(testSource).toBeDefined();
  const diagnostics = [...parsed.errors, ...ts.getPreEmitDiagnostics(program, testSource)];
  expect(
    diagnostics.map((diagnostic) => ts.flattenDiagnosticMessageText(diagnostic.messageText, '\n'))
  ).toEqual([]);
}

test(
  merged_types_type_every_merged_read_and_the_sequencer_receipt,
  merged_types_type_every_merged_read_and_the_sequencer_receipt
);

function merged_types_export_no_joined_anchor_or_custody_type() {
  expect(config.error).toBeUndefined();
  expect(parsed.errors).toEqual([]);
  const indexSource = program.getSourceFile(`${packageDir}src/index.ts`);
  expect(indexSource).toBeDefined();
  const checker = program.getTypeChecker();
  const namespace = checker.getSymbolAtLocation(indexSource!);
  expect(namespace).toBeDefined();
  const exportedNames = checker.getExportsOfModule(namespace!).map((symbol) => symbol.getName());

  expect(exportedNames).toContain('ChainAccount');
  expect(exportedNames).toContain('NetworkHead');
  for (const name of [
    'PaxeerAccountHalf',
    'JoinedAccount',
    'CustodyAssetRecord',
    'JoinedAsset',
    'AssetMap',
    'JoinedBalanceRow',
    'JoinedBalances',
    'CompletionAsset',
    'CompletedBalance',
    'AccountBalances',
    'KernelStatus',
    'AnchorHead',
    'AnchorStatusName',
    'SignCustodyResponse',
  ]) {
    expect(exportedNames).not.toContain(name);
  }
}

test(
  merged_types_export_no_joined_anchor_or_custody_type,
  merged_types_export_no_joined_anchor_or_custody_type
);
