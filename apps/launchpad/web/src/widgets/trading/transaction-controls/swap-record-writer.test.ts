import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { beforeAll, expect, test } from 'vitest';
import {
  createPublicClient,
  createWalletClient,
  defineChain,
  encodeFunctionData,
  erc20Abi,
  getAddress,
  http,
  keccak256,
  parseAbi,
  type Abi,
  type Address,
  type PublicClient,
} from 'viem';
import { CORE_DEPLOYMENT, CORE_TRADE_EXECUTION_ROLES } from '@/core/network/deployment';
import { homeTradeFill } from '@/widgets/trade/home-chain';
import { attemptState, type TxAttempt } from './domain';
import {
  ChainTradeProvider,
  productionTradeDeployment,
  swapRecordWriter,
  tradeRouterAbi,
  transactionFingerprint,
  type TradeDeployment,
} from './providers';

const source = `
pragma solidity ^0.8.20;
contract Token {
    event Transfer(address indexed from, address indexed to, uint256 amount);
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;
    constructor() { balanceOf[msg.sender] = 10**24; }
    function decimals() external pure returns (uint8) { return 6; }
    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount; return true;
    }
    function transfer(address to, uint256 amount) external returns (bool) {
        move(msg.sender, to, amount); return true;
    }
    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        allowance[from][msg.sender] -= amount; move(from, to, amount); return true;
    }
    function move(address from, address to, uint256 amount) internal {
        balanceOf[from] -= amount; balanceOf[to] += amount; emit Transfer(from, to, amount);
    }
}
contract Records {
    event Note(uint256 amount);
    event Swap(bytes32 indexed poolId, address indexed sender, bool isBuy, uint256 amountIn,
        uint256 amountOut, uint256 fee, uint256 price, uint256 timestamp, uint256 blockNumber);
    address public tokenAddress;
    address public registeredPool;
    constructor(address token) { tokenAddress = token; }
    function register(address pool) external { registeredPool = pool; }
    function isRegisteredPool(address pool) external view returns (bool) { return pool == registeredPool; }
    function note(uint256 amount) external { emit Note(amount); }
    function writeSwap(address pool, address sender, bool isBuy, uint256 amountIn, uint256 amountOut) public {
        emit Swap(bytes32(uint256(uint160(pool))), sender, isBuy, amountIn, amountOut, 1, 1, block.timestamp, block.number);
    }
}
contract Router is Records {
    Token immutable quote;
    Token immutable coin;
    Records immutable treasury;
    Records immutable fees;
    Records immutable module;
    address public writer;
    uint256 public count = 1;
    constructor(address q, address t, address tr, address f, address m) Records(t) {
        quote = Token(q); coin = Token(t); treasury = Records(tr); fees = Records(f); module = Records(m);
        writer = m;
    }
    function configure(address nextWriter, uint256 nextCount) external { writer = nextWriter; count = nextCount; }
    function buy(address pool, uint256 amount, uint256 minimum, uint256 deadline) external {
        trade(pool, amount, minimum, deadline, true);
    }
    function sell(address pool, uint256 amount, uint256 minimum, uint256 deadline) external {
        trade(pool, amount, minimum, deadline, false);
    }
    function trade(address pool, uint256 amount, uint256 minimum, uint256 deadline, bool isBuy) internal {
        uint256 output = amount * 2;
        require(deadline >= block.timestamp && output >= minimum);
        if (isBuy) {
            quote.transferFrom(msg.sender, address(this), amount); coin.transfer(msg.sender, output);
        } else {
            quote.transfer(msg.sender, output); coin.transferFrom(msg.sender, address(this), amount);
        }
        treasury.note(1); fees.note(1);
        for (uint256 i = 0; i < count; i++) {
            if (writer == address(this)) writeSwap(pool, msg.sender, isBuy, amount, output);
            else Records(writer).writeSwap(pool, msg.sender, isBuy, amount, output);
        }
        emit Note(amount); module.note(output);
    }
}
`;

type Artifact = { abi: Abi; evm: { bytecode: { object: string } } };
function compileRecords(): Record<string, Artifact> {
  const fixturePath = process.env.L27_PHASE2_FIXTURE;
  if (!fixturePath) throw new Error('Prepared trading deployment required');
  const fixture = JSON.parse(readFileSync(fixturePath, 'utf8'));
  const artifact = fixture.consumedArtifacts[0].path as string;
  const debug = JSON.parse(readFileSync(artifact.replace(/\.json$/, '.dbg.json'), 'utf8'));
  const build = JSON.parse(readFileSync(resolve(dirname(artifact), debug.buildInfo), 'utf8'));
  const cache = join(process.env.XDG_CACHE_HOME ?? join(homedir(), '.cache'), 'hardhat-nodejs/compilers-v2/linux-amd64');
  const list = JSON.parse(readFileSync(join(cache, 'list.json'), 'utf8')) as { builds: { longVersion: string; path: string }[] };
  const matches = list.builds.filter(entry => entry.longVersion === build.solcLongVersion);
  expect(matches).toHaveLength(1);
  const compiled = JSON.parse(execFileSync(join(cache, matches[0].path), ['--standard-json'], {
    input: JSON.stringify({
      language: 'Solidity', sources: { 'Records.sol': { content: source } },
      settings: { optimizer: { enabled: true, runs: 200 }, outputSelection: { '*': { '*': ['abi', 'evm.bytecode.object'] } } },
    }),
    encoding: 'utf8', maxBuffer: 4 * 1024 * 1024,
  })) as { errors?: { severity: string; formattedMessage: string }[]; contracts: Record<string, Record<string, Artifact>> };
  expect(compiled.errors?.filter(error => error.severity === 'error') ?? []).toEqual([]);
  return compiled.contracts['Records.sol'];
}

const configureAbi = parseAbi(['function configure(address writer,uint256 count)']);
let client: PublicClient;
let wallet: ReturnType<typeof createWalletClient>;
let account: Address;
let pool: Address;
let token: Address;
let moduleAddress: Address;
let treasury: Address;
let fees: Address;
let deployment: TradeDeployment;

beforeAll(async () => {
  const rpc = process.env.L26_RPC_URL;
  if (!rpc || !/^http:\/\/127\.0\.0\.1:\d+$/.test(rpc)) throw new Error('Owned loopback RPC required');
  const chain = defineChain({ id: 125, name: 'Swap records', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: [rpc] } } });
  client = createPublicClient({ chain, cacheTime: 0, transport: http(rpc, { retryCount: 0 }) });
  wallet = createWalletClient({ chain, transport: http(rpc, { retryCount: 0 }) });
  [account] = await wallet.getAddresses();
  expect(await client.getChainId()).toBe(125);
  const artifacts = compileRecords();
  async function deploy(name: string, args: readonly unknown[] = []) {
    const artifact = artifacts[name];
    const hash = await wallet.deployContract({ account, chain, abi: artifact.abi, bytecode: `0x${artifact.evm.bytecode.object}`, args });
    const receipt = await client.waitForTransactionReceipt({ hash });
    expect(receipt.status).toBe('success');
    expect(receipt.contractAddress).toBeTruthy();
    return getAddress(receipt.contractAddress!);
  }
  const quote = await deploy('Token');
  token = await deploy('Token');
  pool = await deploy('Records', [token]);
  moduleAddress = await deploy('Records', [token]);
  treasury = await deploy('Records', [token]);
  fees = await deploy('Records', [token]);
  const emitter = await deploy('Records', [token]);
  const quoter = await deploy('Records', [token]);
  const registry = await deploy('Records', [token]);
  const router = await deploy('Router', [quote, token, treasury, fees, moduleAddress]);
  async function send(address: Address, abi: Abi, functionName: string, args: readonly unknown[]) {
    const hash = await wallet.writeContract({ account, chain, address, abi, functionName, args });
    expect((await client.waitForTransactionReceipt({ hash })).status).toBe('success');
  }
  await send(registry, artifacts.Records.abi, 'register', [pool]);
  for (const address of [quote, token]) {
    await send(address, erc20Abi, 'transfer', [router, 10n ** 15n]);
    await send(address, erc20Abi, 'approve', [router, 10n ** 15n]);
  }
  const pins = await Promise.all([router, quoter, registry, emitter].map(async address => {
    const code = await client.getCode({ address });
    expect(code).toBeTruthy();
    return { address, runtimeHash: keccak256(code!) };
  }));
  deployment = { revision: 'swap-records', router, spender: moduleAddress, quoter, registry, emitter, swapRecordWriter: moduleAddress, usdl: quote, pins };
  expect(deployment.pins.some(pin => pin.address === moduleAddress)).toBe(false);
}, 60_000);

async function minedTrade(isBuy: boolean, writer: Address, count = 1) {
  const chain = wallet.chain!;
  const configured = await wallet.writeContract({ account, chain, address: deployment.router, abi: configureAbi, functionName: 'configure', args: [writer, BigInt(count)] });
  expect((await client.waitForTransactionReceipt({ hash: configured })).status).toBe('success');
  const amount = 5_000_000n, output = amount * 2n, minimum = output - 1n;
  const block = await client.getBlock();
  const data = encodeFunctionData({ abi: tradeRouterAbi, functionName: isBuy ? 'buy' : 'sell', args: [pool, amount, minimum, block.timestamp + 600n] });
  const hash = await wallet.sendTransaction({ account, chain, to: deployment.router, data });
  const receipt = await client.waitForTransactionReceipt({ hash });
  expect(receipt.status).toBe('success');
  const attempt: TxAttempt = { intentId: hash, chainId: 125, from: account, txHash: hash, submittedAt: Date.now(), action: isBuy ? 'buy' : 'sell', actionFingerprint: transactionFingerprint(deployment.router, data) };
  return { receipt, attempt, amount, output, minimum, isBuy };
}

async function assertConfirmed(provider: ChainTradeProvider, trade: Awaited<ReturnType<typeof minedTrade>>) {
  const evidence = await provider.readReceipt(trade.attempt);
  expect(evidence).toEqual({ transactionHash: trade.receipt.transactionHash, status: 'success', blockNumber: trade.receipt.blockNumber });
  expect(attemptState(trade.attempt, Date.now(), evidence)).toBe('confirmed_success');
  expect(provider.fills.get(trade.attempt.txHash.toLowerCase())).toEqual({ inputRaw: trade.amount, outputRaw: trade.output, feeRaw: 1n, feeAsset: { address: trade.isBuy ? deployment.usdl : token, decimals: 6 }, blockHeight: trade.receipt.blockNumber, isBuy: trade.isBuy, pool, token, tokenDecimals: 6 });
  const fill = homeTradeFill(trade.receipt, { emitter: swapRecordWriter(provider.deployment), pool, account, isBuy: trade.isBuy, amount: trade.amount, minimum: trade.minimum });
  expect(fill.amountIn).toBe(trade.amount);
  expect(fill.amountOut).toBe(trade.output);
}

async function swapRecordIsReadFromTheLaunchpadModule() {
  const provider = new ChainTradeProvider(client, undefined, deployment);
  expect(swapRecordWriter(deployment)).toBe(moduleAddress);
  for (const isBuy of [true, false]) {
    const trade = await minedTrade(isBuy, moduleAddress);
    expect(trade.receipt.logs.map(log => log.address.toLowerCase())).toEqual([deployment.usdl, token, treasury, fees, moduleAddress, deployment.router, moduleAddress].map(address => address.toLowerCase()));
    expect(trade.receipt.logs).toHaveLength(7);
    expect(trade.receipt.logs.some(log => log.address.toLowerCase() === deployment.emitter.toLowerCase())).toBe(false);
    await assertConfirmed(provider, trade);
  }
}

async function swapRecordFromAnotherAddressIsNotCounted() {
  const provider = new ChainTradeProvider(client, undefined, deployment);
  for (const isBuy of [true, false]) {
    for (const [writer, count] of [[deployment.emitter, 1], [deployment.router, 1], [moduleAddress, 2]] as const) {
      const trade = await minedTrade(isBuy, writer, count);
      await expect(provider.readReceipt(trade.attempt)).rejects.toThrow('swap_evidence_missing_or_duplicate');
      expect(provider.fills.has(trade.attempt.txHash.toLowerCase())).toBe(false);
    }
  }
}

async function deploymentWithoutAWriterKeepsTheEventEmitter() {
  const { swapRecordWriter: writer, ...legacy } = deployment;
  expect(writer).toBe(moduleAddress);
  expect(swapRecordWriter(legacy)).toBe(legacy.emitter);
  const provider = new ChainTradeProvider(client, undefined, legacy);
  for (const isBuy of [true, false]) await assertConfirmed(provider, await minedTrade(isBuy, legacy.emitter));
}

function productionRecordNamesTheLaunchpadModule() {
  expect(productionTradeDeployment.swapRecordWriter).toBe(CORE_DEPLOYMENT.launchpadPrecompile);
  expect(swapRecordWriter(productionTradeDeployment)).toBe(CORE_DEPLOYMENT.launchpadPrecompile);
  expect(productionTradeDeployment.pins.map(pin => pin.address)).toEqual(CORE_TRADE_EXECUTION_ROLES.map(role => CORE_DEPLOYMENT[role]));
  expect(productionTradeDeployment.pins.some(pin => pin.address === CORE_DEPLOYMENT.launchpadPrecompile)).toBe(false);
  const provider = new ChainTradeProvider(client, undefined, productionTradeDeployment);
  expect(provider.deployment).toBe(productionTradeDeployment);
  expect(() => new ChainTradeProvider(client, undefined, { ...deployment, pins: deployment.pins.filter(pin => pin.address !== deployment.emitter) })).toThrow('missing_deployment_code_pin');
}

test(swapRecordIsReadFromTheLaunchpadModule.name, swapRecordIsReadFromTheLaunchpadModule, 30_000);
test(swapRecordFromAnotherAddressIsNotCounted.name, swapRecordFromAnotherAddressIsNotCounted, 30_000);
test(deploymentWithoutAWriterKeepsTheEventEmitter.name, deploymentWithoutAWriterKeepsTheEventEmitter, 30_000);
test(productionRecordNamesTheLaunchpadModule.name, productionRecordNamesTheLaunchpadModule);
