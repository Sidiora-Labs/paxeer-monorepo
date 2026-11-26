import { createServer } from 'node:http';
import { expect } from 'vitest';
import { decodeFunctionData, recoverAddress, serializeSignature, type Hex } from 'viem';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import type { SponsoredBindRequest } from '../../src/provision/bind.js';
import { ADDR, TestChain, addrAbiJson } from './chain.js';
import { CHAIN_ID, PAYMASTER } from '../e2e/attestors.js';

export async function startBindingStation(
  chain: TestChain,
  options: { tokenAmount?: string } = {},
) {
  const sponsor = privateKeyToAccount(generatePrivateKey());
  const sponsorAddress = sponsor.address.toLowerCase();
  chain.balances.set(sponsorAddress, 10n ** 21n);
  type QuoteRequest = Omit<SponsoredBindRequest['batch'], 'quote'> & {
    token: Hex;
    maxTokenAmount: string;
    gasCost: string;
    decimals: 6;
  };
  let issued: SponsoredBindRequest['batch'] | undefined;
  const station = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (chunk: Buffer) => chunks.push(chunk));
    req.on('end', () => {
      void (async () => {
        const { constructionDigest, sponsorQuoteDigest } = await import(
          '../../src/attestor/client.js'
        );
        const { sponsoredBindAbi, SIDIORA_TOKEN } = await import('../../src/provision/bind.js');
        const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
        expect(req.method).toBe('POST');
        expect(req.headers['content-type']).toBe('application/json');
        if (req.url === '/quote') {
          const request: QuoteRequest = body;
          expect(request).toMatchObject({
            chainId: String(CHAIN_ID),
            nonce: '0',
            token: SIDIORA_TOKEN,
            decimals: 6,
            maxTokenAmount: '1000000',
          });
          expect(request.calls).toHaveLength(1);
          expect(request.calls[0]!.to).toBe(ADDR);
          expect(request.calls[0]!.value).toBe('0');
          const quote: SponsoredBindRequest['batch']['quote'] = {
            sponsor: sponsor.address,
            token: request.token,
            maxTokenAmount: request.maxTokenAmount,
            tokenAmount: options.tokenAmount ?? '100',
            deadline: String(Math.floor(Date.now() / 1000) + 300),
            quoteNonce: '1',
            gasCost: request.gasCost,
            decimals: 6,
          };
          issued = {
            chainId: request.chainId,
            account: request.account,
            nonce: request.nonce,
            calls: request.calls,
            quote,
          };
          const relayerSignature = await sponsor.sign({
            hash: sponsorQuoteDigest({ kind: 'sponsored_batch', ...issued }),
          });
          res.writeHead(200, { 'content-type': 'application/json' });
          res.end(JSON.stringify({ quote, relayerSignature }));
          return;
        }
        expect(req.url).toBe('/submit');
        const request: SponsoredBindRequest = body;
        expect(issued).toBeDefined();
        expect(request.batch).toEqual(issued);
        expect(
          await recoverAddress({
            hash: constructionDigest({ kind: 'sponsored_batch', ...request.batch }),
            signature: request.accountSignature,
          }),
        ).toBe(request.batch.account);
        expect(
          await recoverAddress({
            hash: sponsorQuoteDigest({ kind: 'sponsored_batch', ...request.batch }),
            signature: request.relayerSignature as Hex,
          }),
        ).toBe(sponsor.address);
        const authorization = request.authorization;
        expect(authorization).toMatchObject({
          chainId: String(CHAIN_ID),
          address: PAYMASTER,
          nonce: '0',
        });
        expect(
          await recoverAddress({
            hash: constructionDigest({
              kind: 'eip7702_authorization',
              chainId: authorization.chainId,
              address: authorization.address,
              nonce: authorization.nonce,
            }),
            signature: serializeSignature(authorization),
          }),
        ).toBe(request.batch.account);
        const decoded = decodeFunctionData({
          abi: sponsoredBindAbi,
          data: request.call.data,
        });
        expect(decoded.functionName).toBe('executeSponsored');
        expect(decoded.args![0]).toEqual(
          request.batch.calls.map((call) => ({ ...call, value: BigInt(call.value) })),
        );
        expect(decoded.args![2]).toBe(request.accountSignature);
        expect(decoded.args![3]).toBe(request.relayerSignature);
        expect(request.call.to).toBe(request.batch.account);
        expect(request.call.value).toBe('0');
        const { sharedRpcPool } = await import('../../src/rpc/pool.js');
        const rpc = sharedRpcPool();
        const raw = await sponsor.signTransaction({
          type: 'eip7702',
          chainId: CHAIN_ID,
          nonce: await rpc.getTransactionCount(sponsor.address, 'latest'),
          to: request.call.to,
          value: 0n,
          data: request.call.data,
          gas: 200_000n,
          maxFeePerGas: BigInt(request.batch.quote.gasCost) / 200_000n,
          maxPriorityFeePerGas: chain.tip,
          authorizationList: [
            {
              ...authorization,
              chainId: Number(authorization.chainId),
              nonce: Number(authorization.nonce),
            },
          ],
        });
        const transactionHash = await rpc.sendRawTransaction(raw);
        const binding = decodeFunctionData({
          abi: addrAbiJson,
          data: request.batch.calls[0]!.data,
        });
        expect(binding.functionName).toBe('bindLayerX');
        expect(chain.bindings.get(request.batch.account.toLowerCase())).toBe(
          (binding.args![0] as Hex).slice(2),
        );
        expect(chain.bindNonces.get(request.batch.account.toLowerCase())).toBe(1n);
        res.writeHead(200, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ transactionHash }));
      })().catch((error: Error) => {
        res.writeHead(500, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ error: 'station_contract_violation', message: error.message }));
      });
    });
  });
  await new Promise<void>((resolve, reject) => {
    station.once('error', reject);
    station.listen(0, '127.0.0.1', resolve);
  });
  const stationAddress = station.address();
  if (!stationAddress || typeof stationAddress === 'string')
    throw new Error('station did not bind');

  return { server: station, sponsorAddress, url: `http://127.0.0.1:${stationAddress.port}` };
}
