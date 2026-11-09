import { createHash } from 'node:crypto';
import { DirectSecp256k1HdWallet } from '@cosmjs/proto-signing';
import { SigningCosmWasmClient } from '@cosmjs/cosmwasm-stargate';
import { stringToPath } from '@cosmjs/crypto';
import { TxRaw } from 'cosmjs-types/cosmos/tx/v1beta1/tx';
import { ethers } from 'ethers';
import { expect } from 'chai';
import { bothProviders, rawPax, rawGeth, expectJsonRpcError, sleep, waitUntil } from '../utils/chainUtils';
import { readRuntimeState, RuntimeState, claimPool, expectSameError } from '../utils/testUtils';
import { EvmAccount, fundFromUnlocked } from '../utils/evmUtils';
import { generatePaxAddress } from '../utils/cosmosUtils';
import { Endpoints } from '../config/endpoints';
import { PAX_HD_PATH, EXEC_FEE } from '../utils/constants';
import { sharedRichBlock, sendSingleTx, RichBlock, SentTx, richFailedTxs, blockReceipts, txCountByNumber, assertTxCount, findEmptyBlock } from '../utils/txUtils';

const wasmMode = process.env.RPC_TESTS_WASM_MODE ?? 'enabled';

describe('eth_getBlockTransactionCountByNumber', function () {
    this.timeout(300 * 1000);

    const { pax, geth } = bothProviders();

    let runtime: RuntimeState;
    let richPax: RichBlock;
    let paxOne: { number: number; hash: string; tx: SentTx };
    let gethOne: { number: number; hash: string; tx: SentTx };
    let gethSigner: EvmAccount;
    let emptyBlock: { number: number; hash: string } | undefined;

    before(async function () {
        this.timeout(300 * 1000);
        await sleep(5000);
        runtime = readRuntimeState();
        expect(wasmMode, 'RPC_TESTS_WASM_MODE must declare enabled or disabled').to.be.oneOf([
            'enabled', 'disabled',
        ]);
        const readWasmParam = async (key: string) => {
            const response = await fetch(`${Endpoints.pax.rest}/cosmos/params/v1beta1/params?subspace=wasm&key=${key}`, {
                signal: AbortSignal.timeout(10_000),
            });
            expect(response.ok, `Wasm ${key} capability query succeeded`).to.equal(true);
            const result = await response.json();
            if (
                typeof result !== 'object' || result === null || !('param' in result) ||
                typeof result.param !== 'object' || result.param === null ||
                !('value' in result.param) || typeof result.param.value !== 'string'
            ) {
                throw new Error(`Wasm ${key} capability query returned an invalid parameter`);
            }
            return JSON.parse(result.param.value);
        };
        const [upload, instantiate] = await Promise.all([
            readWasmParam('uploadAccess'), readWasmParam('instantiateAccess'),
        ]);
        expect(upload.permission, 'upload permission is declared').to.be.oneOf(['Everybody', 'Nobody', 'OnlyAddress']);
        expect(instantiate, 'instantiate permission is declared').to.be.oneOf(['Everybody', 'Nobody', 'OnlyAddress']);
        expect(
            upload.permission === 'Everybody' && instantiate === 'Everybody',
            'the declared Wasm mode agrees with the chain',
        ).to.equal(wasmMode === 'enabled');
        if (wasmMode === 'enabled') {
            expect(runtime.wasm, 'Wasm-enabled qualification requires bootstrap CW20 fixtures').to.not.equal(undefined);
            expect(runtime.wasm!.cw20Pointer, 'Wasm-enabled pointer address').to.match(/^0x[0-9a-fA-F]{40}$/);
            expect(runtime.wasm!.actor.privateKey, 'Wasm-enabled actor key').to.match(/^0x[0-9a-fA-F]{64}$/);
        } else {
            expect(runtime.wasm, 'disabled-Wasm qualification has no CW20 fixtures').to.equal(undefined);
        }
        const [paxOneSigner] = claimPool(runtime, pax, 1, 'eth_getBlockTransactionCountByNumber');

        const gethDev: string = (await geth.send('eth_accounts', []))[0];
        gethSigner = EvmAccount.fromPrivateKey(ethers.Wallet.createRandom().privateKey, geth);
        await fundFromUnlocked(geth, gethDev, gethSigner.address, ethers.parseEther('10'));

        richPax = await sharedRichBlock(pax, runtime);
        paxOne = await sendSingleTx(pax, paxOneSigner);
        gethOne = await sendSingleTx(geth, gethSigner);
        emptyBlock = await findEmptyBlock(pax);
    });

    describe('counts agree with every other view of the block', () => {
        it('counts every transaction in the rich block including the failed txs', async () => {
            const count = await txCountByNumber(pax, richPax.number);
            assertTxCount(count, richPax.txs.length, 'rich block tx count');
        });

        it('equals the eth_getBlockByNumber tx list length and the receipt count', async () => {
            const [count, block, receipts] = await Promise.all([
                txCountByNumber(pax, richPax.number),
                pax.send('eth_getBlockByNumber', [ethers.toQuantity(richPax.number), false]),
                blockReceipts(pax, richPax.number),
            ]);
            assertTxCount(count, block.transactions.length, 'count == block.transactions');
            expect(receipts.length, 'count == receipts length').to.equal(Number(BigInt(count)));
        });

        it('a known empty block reports 0x0', async function () {
            if (!emptyBlock) this.skip();
            const count = await txCountByNumber(pax, emptyBlock!.number);
            expect(count, 'empty block count is exactly 0x0').to.equal('0x0');
            const block = await pax.send('eth_getBlockByNumber', [
                ethers.toQuantity(emptyBlock!.number),
                false,
            ]);
            expect(block.transactions.length, 'block really is empty').to.equal(0);
        });
    });

    describe('block tags resolve', () => {
        it('earliest (genesis) has no transactions', async () => {
            expect(await txCountByNumber(pax, 'earliest')).to.equal('0x0');
        });

        it('latest matches the head block tx list', async () => {
            const head = await pax.send('eth_getBlockByNumber', ['latest', false]);
            expect(head, 'latest resolves to a committed block').to.not.equal(null);
            const height = Number(BigInt(head.number));
            const count = await txCountByNumber(pax, height);
            assertTxCount(count, head.transactions.length, `count at pinned head ${height}`);
        });

        it('pending returns a canonical quantity', async () => {
            const count = await txCountByNumber(pax, 'pending');
            assertTxCount(count, Number(BigInt(count)), 'pending count');
        });
    });

    describe('geth parity', () => {
        it('a single-transaction block counts 0x1 on both chains', async () => {
            const [s, g] = await Promise.all([
                txCountByNumber(pax, paxOne.number),
                txCountByNumber(geth, gethOne.number),
            ]);
            expect(s, 'Pax single-tx count').to.equal('0x1');
            expect(g, 'geth single-tx count').to.equal('0x1');
        });
    });

    async function sharedExclusionBlock(kind: 'bank' | 'cw20') {
        const [evmSigner] = claimPool(runtime, pax, 1, `txcount-shared-${kind}`);
        const wallet = await DirectSecp256k1HdWallet.fromMnemonic(runtime.funded.adminMnemonic, {
            prefix: 'pax', hdPaths: [stringToPath(PAX_HD_PATH)],
        });
        const [sender] = await wallet.getAccounts();
        const recipient = await generatePaxAddress();
        const wasm = runtime.wasm;
        if (kind === 'cw20' && !wasm) throw new Error('CW20 shared-block fixture requires enabled Wasm');
        const message = kind === 'bank'
            ? {
                typeUrl: '/cosmos.bank.v1beta1.MsgSend',
                value: { fromAddress: sender.address, toAddress: recipient, amount: [{ denom: 'uhpx', amount: '222111' }] },
            }
            : {
                typeUrl: '/cosmwasm.wasm.v1.MsgExecuteContract',
                value: {
                    sender: sender.address, contract: wasm!.cw20, funds: [],
                    msg: Buffer.from(JSON.stringify({ transfer: { recipient, amount: '1' } })),
                },
            };
        const client = await SigningCosmWasmClient.connectWithSigner(Endpoints.pax.cosmosRpc, wallet);
        const attempts: string[] = [];
        try {
            for (let attempt = 0; attempt < 4; attempt++) {
                const [nativeTx, evmRequest] = await Promise.all([
                    client.sign(sender.address, [message], EXEC_FEE, `rpc_tests shared ${kind}`),
                    evmSigner.wallet.populateTransaction({ to: evmSigner.address, value: 0n, gasLimit: 21000n }),
                ]);
                const nativeRaw = TxRaw.encode(nativeTx).finish();
                const evmRaw = await evmSigner.wallet.signTransaction(evmRequest);
                const observed = BigInt(await pax.send('eth_blockNumber', []));
                await waitUntil(async () => {
                    const head = BigInt(await pax.send('eth_blockNumber', []));
                    return head > observed ? head : null;
                }, { timeoutMs: 30_000, intervalMs: 100, label: `fresh head for shared ${kind} block` });
                const [native, evmTx] = await Promise.all([
                    client.broadcastTx(nativeRaw, 30_000, 100),
                    pax.broadcastTransaction(evmRaw),
                ]);
                expect(native.code, `native ${kind} execution succeeds: ${native.rawLog}`).to.equal(0);
                const receipt = await evmTx.wait(1, 30_000);
                expect(receipt, 'EVM fixture committed').to.not.equal(null);
                expect(receipt!.status, 'EVM fixture execution succeeds').to.equal(1);
                attempts.push(`native=${native.height}, EVM=${receipt!.blockNumber}`);
                if (native.height !== receipt!.blockNumber) continue;
                const consensusBlock = await client.getBlock(native.height);
                expect(
                    consensusBlock.txs.map(tx => createHash('sha256').update(tx).digest('hex').toUpperCase()),
                    'the native fixture is present in the shared consensus block',
                ).to.include(native.transactionHash.toUpperCase());
                return { height: native.height, nativeHash: '0x' + native.transactionHash.toLowerCase(), evmHash: evmTx.hash };
            }
            throw new Error(`required shared ${kind}/EVM block not established: ${attempts.join('; ')}`);
        } finally {
            client.disconnect();
        }
    }

    describe('dual-VM: a Cosmos bank send is not counted', () => {
        let fixture: Awaited<ReturnType<typeof sharedExclusionBlock>>;

        before(async function () {
            this.timeout(180 * 1000);
            fixture = await sharedExclusionBlock('bank');
        });

        it('the count equals the EVM tx count, excluding the Cosmos tx', async () => {
            const [count, block] = await Promise.all([
                txCountByNumber(pax, fixture.height),
                pax.send('eth_getBlockByNumber', [ethers.toQuantity(fixture.height), false]),
            ]);
            const hashes = block.transactions.map((h: string) => h.toLowerCase());
            expect(hashes, 'EVM tx present').to.include(fixture.evmHash.toLowerCase());
            expect(hashes, 'Cosmos tx not in the EVM block').to.not.include(fixture.nativeHash);
            assertTxCount(count, block.transactions.length, 'count == EVM tx count');
        });
    });

    if (wasmMode === 'enabled') {
        describe('dual-VM (Wasm enabled): a pointer-backed CW20 wasm transfer is not counted', () => {
            let fixture: Awaited<ReturnType<typeof sharedExclusionBlock>>;

            before(async function () {
                this.timeout(180 * 1000);
                fixture = await sharedExclusionBlock('cw20');
            });

            it('the count equals the EVM tx count, excluding the pointer-backed CW20 tx', async () => {
                const [count, block] = await Promise.all([
                    txCountByNumber(pax, fixture.height),
                    pax.send('eth_getBlockByNumber', [ethers.toQuantity(fixture.height), false]),
                ]);
                const hashes = block.transactions.map((h: string) => h.toLowerCase());
                expect(hashes, 'EVM tx present').to.include(fixture.evmHash.toLowerCase());
                expect(hashes, 'CW20 wasm tx not in the EVM block').to.not.include(fixture.nativeHash);
                assertTxCount(count, block.transactions.length, 'count == EVM tx count');
            });
        });
    } else {
        describe('explicit disabled-Wasm mode', () => {
            it('qualifies native/EVM exclusion without claiming CW20 coverage', () => {
                expect(wasmMode).to.equal('disabled');
                expect(runtime.wasm).to.equal(undefined);
            });
        });
    }

    describe('wrong params / error handling (parity with geth)', () => {
        it('empty params fail identically (-32602, missing argument 0)', async () => {
            const [s, g] = await Promise.all([
                rawPax('eth_getBlockTransactionCountByNumber', []),
                rawGeth('eth_getBlockTransactionCountByNumber', []),
            ]);
            expectJsonRpcError(s, -32602, /missing value for required argument 0/);
            expectSameError(s, g);
        });

        it('too many positional args fail identically (-32602, want at most 1)', async () => {
            const [s, g] = await Promise.all([
                rawPax('eth_getBlockTransactionCountByNumber', ['latest', 1]),
                rawGeth('eth_getBlockTransactionCountByNumber', ['latest', 1]),
            ]);
            expectJsonRpcError(s, -32602, /too many arguments, want at most 1/);
            expectSameError(s, g);
        });

        it('a far-future block returns null on both chains', async () => {
            const future = ethers.toQuantity((await pax.getBlockNumber()) + 10_000_000);
            const [s, g] = await Promise.all([
                rawPax('eth_getBlockTransactionCountByNumber', [future]),
                rawGeth('eth_getBlockTransactionCountByNumber', [future]),
            ]);
            // A not-yet-mined height has no count: both return a null result, not an error.
            expect(g.error, 'geth does not error on a future block').to.equal(undefined);
            expect(g.result, 'geth returns null for a future block').to.equal(null);
            expect(s.error, 'Pax does not error on a future block').to.equal(undefined);
            expect(s.result, 'Pax returns null for a future block').to.equal(null);
        });
    });
});
