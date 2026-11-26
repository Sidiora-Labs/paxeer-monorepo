import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { randomUUID } from 'node:crypto';

export async function startChainData(chain) {
  const reads = [];
  const { ethers, provider, registry, router, artifact, usdl } = chain;
  const lower = (value) => value.toLowerCase();
  async function snapshot() {
    const block = await provider.getBlock('latest');
    const at = new Date(Number(block.timestamp) * 1000).toISOString();
    const watermark = {
      chainId: '125',
      height: String(block.number),
      blockHash: block.hash,
      integrity: 'healthy',
      observedAt: at,
      projectionRevision: String(block.number),
    };
    const envelope = (data) => ({
      data,
      generatedAt: at,
      nextCursor: null,
      quality: 'complete',
      requestId: randomUUID(),
      revision: String(block.number),
      watermark,
    });
    return { block, at, watermark, envelope };
  }
  const sourcePosition = (log) => ({
    chainId: '125',
    height: String(log.blockNumber),
    blockHash: log.blockHash,
    transactionHash: log.transactionHash,
    transactionIndex: String(log.transactionIndex),
    logIndex: String(log.index),
  });
  async function detail(address, snap) {
    assert.equal(await registry.isRegisteredPool(address, { blockTag: snap.block.number }), true);
    const pool = new ethers.Contract(address, artifact('SidioraPool').abi, provider);
    const meta = await registry.getPoolMetadata(address, { blockTag: snap.block.number });
    const token = new ethers.Contract(meta.token, artifact('SidioraERC20').abi, provider);
    const options = { blockTag: snap.block.number };
    const [reserves, price, supply, decimals, name, symbol, buys, sells] = await Promise.all([
      pool.getReserves(options),
      pool.getPrice(options),
      token.totalSupply(options),
      token.decimals(options),
      token.name(options),
      token.symbol(options),
      router.queryFilter(router.filters.Buy(address), 0, snap.block.number),
      router.queryFilter(router.filters.Sell(address), 0, snap.block.number),
    ]);
    const [virtual, real, tokenReserve] = reserves;
    const volume =
      buys.reduce((sum, log) => sum + log.args[2], 0n) +
      sells.reduce((sum, log) => sum + log.args[3], 0n);
    const created = await registry.queryFilter(
      registry.filters.PoolRegistered(address),
      Number(meta.createdBlock),
      Number(meta.createdBlock),
    );
    assert.equal(created.length, 1);
    const createdPosition = sourcePosition(created[0]);
    const lastEvent = [...created, ...buys, ...sells]
      .sort(
        (a, b) =>
          a.blockNumber - b.blockNumber ||
          a.transactionIndex - b.transactionIndex ||
          a.index - b.index,
      )
      .at(-1);
    const usdlPrice = (price * 10n ** decimals) / 10n ** 18n;
    return {
      pool: {
        chainId: '125',
        homeChain: '0',
        poolAddress: lower(address),
        tokenAddress: lower(meta.token),
        creator: lower(meta.creator),
        createdAt: new Date(Number(meta.createdAt) * 1000).toISOString(),
        createdPosition,
        virtualUsdlReserveRaw: String(virtual),
        realUsdlBalanceRaw: String(real),
        effectivePricingUsdlReserveRaw: String(virtual + real),
        tokenReserveRaw: String(tokenReserve),
        quoteAssetAddress: lower(usdl.target),
        quoteAssetDecimals: 6,
        feeBps: Number(await pool.currentFeeBps(options)),
      },
      token: {
        address: lower(meta.token),
        name,
        symbol,
        decimals: Number(decimals),
        decimalsQuality: 'verified',
        logoAssetId: null,
        metadataRevision: '1',
        sourcePosition: createdPosition,
        supplyQuality: 'verified',
        totalSupplyRaw: String(supply),
      },
      stats: {
        poolAddress: lower(address),
        buys24h: String(buys.length),
        sells24h: String(sells.length),
        changeBaselineQuality: 'insufficient_history',
        circulatingMarketCapUsdlRaw: null,
        holderCount: null,
        indicativeTvlUsdlRaw: null,
        priceChangeBps24h: null,
        realUsdlBalanceRaw: String(real),
        sourceUpperPosition: sourcePosition(lastEvent),
        supplyValuationQuality: 'verified',
        totalSupplyValuationUsdlRaw: String((price * supply) / 10n ** 18n),
        valuationChangeUsdlRaw: null,
        valuationMethod: null,
        volume24hRaw: String(volume),
        watermark: snap.watermark,
        windowStart: new Date(Number(snap.block.timestamp - 86400) * 1000).toISOString(),
        windowEnd: snap.at,
        spotPriceUsdlRaw: String(usdlPrice),
      },
    };
  }
  const server = createServer((request, response) => {
    void (async () => {
      assert.equal(request.method, 'GET');
      assert.equal(request.headers.authorization, undefined);
      assert.equal(request.headers['x-agent-signature'], undefined);
      const url = new URL(request.url, 'http://localhost');
      const snap = await snapshot();
      const pools = await registry.getAllPools(0, 100, { blockTag: snap.block.number });
      let body;
      if (url.pathname === '/v2/pools') {
        const details = await Promise.all(pools.map((address) => detail(address, snap)));
        body = snap.envelope({ items: details.map((value) => value.pool) });
      } else if (/^\/v2\/pools\/0x[0-9a-f]{40}$/.test(url.pathname)) {
        body = snap.envelope(await detail(url.pathname.split('/')[3], snap));
      } else if (/^\/v3\/wallets\/0x[0-9a-f]{40}\/portfolio$/.test(url.pathname)) {
        const wallet = url.pathname.split('/')[3];
        const positions = [];
        for (const address of pools) {
          const info = await detail(address, snap);
          const token = new ethers.Contract(
            info.token.address,
            artifact('SidioraERC20').abi,
            provider,
          );
          const balance = await token.balanceOf(wallet, { blockTag: snap.block.number });
          if (balance > 0n)
            positions.push({
              positionId: `125:${info.token.address}`,
              wallet,
              chainId: '125',
              tokenAddress: info.token.address,
              balanceRaw: String(balance),
              knownQuantityRaw: '0',
              unknownQuantityRaw: String(balance),
              basisCoverage: { numerator: '0', denominator: String(balance) },
              basisQuality: 'unknown',
              realizedBasisQuality: 'unknown',
              knownRealizedSubtotalUsdlRaw: '0',
              knownRemainingCostUsdlRaw: '0',
              markPrice: null,
              pricingQuality: 'unpriced',
              realizedPnlRaw: null,
              remainingCostUsdlRaw: null,
              unrealizedPnlRaw: null,
              sourceWatermark: snap.watermark,
            });
        }
        body = {
          version: 3,
          algorithm: 'fifo',
          quoteAsset: usdl.target,
          decimals: 6,
          positionErrors: {},
          data: {
            positions,
            summary: {
              wallet,
              basisQuality: 'unknown',
              completeValueUsdlRaw: null,
              knownRealizedSubtotalUsdlRaw: '0',
              positionCount: positions.length,
              pricedSubtotalUsdlRaw: '0',
              pricingQuality: 'unpriced',
              realizedPnlRaw: null,
              snapshotId: String(snap.block.number),
              unpricedAssetCount: positions.length,
              watermark: snap.watermark,
            },
          },
        };
      } else throw new Error(`Unsupported chain data read ${url.pathname}`);
      reads.push({ route: url.pathname, height: String(snap.block.number), body });
      response.writeHead(200, {
        'content-type': 'application/json',
        'x-schema-version': url.pathname.startsWith('/v3') ? '3' : '2',
      });
      response.end(JSON.stringify(body));
    })().catch((error) => {
      response.writeHead(500, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ error: error.message }));
    });
  });
  await new Promise((done) => server.listen(0, '127.0.0.1', done));
  return {
    url: `http://127.0.0.1:${server.address().port}`,
    reads,
    stop: () =>
      new Promise((done, failed) => server.close((error) => (error ? failed(error) : done()))),
  };
}
