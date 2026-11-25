import { setTimeout } from 'node:timers/promises';

export async function observeOraclePublication(provider, transaction) {
  for (;;) {
    const receipt = await provider.send('eth_getTransactionReceipt', [transaction.hash]);
    if (receipt) {
      const canonical = await provider.send('eth_getBlockByNumber', [
        receipt.blockNumber,
        false,
      ]);
      if (!canonical || canonical.hash !== receipt.blockHash) {
        return { state: 'reorganized', transactionHash: transaction.hash };
      }
      if (receipt.transactionHash !== transaction.hash) {
        throw new Error('Oracle publication identity differs');
      }
      if (receipt.status !== '0x1') throw new Error('Oracle publication reverted');
      return { state: 'confirmed', receipt };
    }
    const pending = await provider.send('eth_getTransactionByHash', [transaction.hash]);
    if (!pending) return { state: 'reorganized', transactionHash: transaction.hash };
    if (pending.hash !== transaction.hash) throw new Error('Oracle publication identity differs');
    await setTimeout(250);
  }
}
