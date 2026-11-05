import assert from 'node:assert/strict';
import { readFile, writeFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from '../../../../apps/wallet/node_modules/vite/dist/node/index.js';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../../../..');
const require = createRequire(resolve(root, 'sdk/wallet/package.json'));
const { ed25519 } = require('@noble/curves/ed25519.js');
const { sha256 } = require('@noble/hashes/sha2.js');
const { bytesToHex, hexToBytes } = require('viem');
const server = await createServer({
  root,
  configFile: false,
  optimizeDeps: { noDiscovery: true, entries: [] },
  server: { middlewareMode: true, hmr: false },
  resolve: {
    alias: {
      '@sidiora/layerx-sdk/browser': resolve(root, 'services/agent/sdk/typescript/src/browser.ts'),
    },
  },
});

try {
  const { encodeKernelReceipt, decodeSequencerReceipt } = await server.ssrLoadModule(
    '/sdk/wallet/src/receipt.ts'
  );
  const { verifySequencerReceipt } = await server.ssrLoadModule(
    '/services/agent/sdk/typescript/src/verifier.ts'
  );
  const source = JSON.parse(
    await readFile(new URL('./sequencer-receipt-v1.json', import.meta.url), 'utf8')
  );
  const cases = [];
  const receipts = new Map();
  for (const entry of source.cases.filter((entry) => /^V[12]-/.test(entry.id))) {
    const pin = entry.pinnedKey;
    const { receipt } = await verifySequencerReceipt(
      hexToBytes(entry.signedReceipt),
      hexToBytes(entry.rawTransaction),
      [
        {
          keyId: hexToBytes(pin.keyId),
          publicKey: hexToBytes(pin.publicKey),
          first_height: BigInt(pin.firstHeight),
          last_height: BigInt(pin.lastHeight),
        },
      ]
    );
    const signedReceipt = bytesToHex(encodeKernelReceipt(receipt));
    assert.equal(signedReceipt, entry.signedReceipt);
    const emitted = structuredClone(entry);
    emitted.signedReceipt = emitted.receiptJson.receipt = signedReceipt;
    emitted.receiptDigest = bytesToHex(
      sha256(Buffer.concat([Buffer.from('LXP/v1/receipt\0'), encodeKernelReceipt(receipt, false)]))
    );
    assert.equal(emitted.receiptDigest, entry.receiptDigest);
    decodeSequencerReceipt(emitted.receiptJson, emitted.receiptJson.txHash, [
      {
        ...pin,
        firstHeight: BigInt(pin.firstHeight),
        lastHeight: BigInt(pin.lastHeight),
      },
    ]);
    cases.push(emitted);
    receipts.set(entry.id, receipt);
  }

  const base = cases.find((entry) => entry.id === 'V1-erc20-approve');
  assert.ok(base);
  // Public conformance signing seed, matching the original kernel vectors.
  const seed = new Uint8Array(32).fill(0x5e);
  assert.equal(bytesToHex(ed25519.getPublicKey(seed)), base.pinnedKey.publicKey);
  function negative(id, description, changeReceipt, changeAnswer) {
    const entry = structuredClone(base);
    const receipt = structuredClone(receipts.get(base.id));
    entry.id = id;
    entry.vector = 'V3';
    entry.description = description;
    if (changeReceipt) changeReceipt(receipt, entry);
    const digest = sha256(
      Buffer.concat([Buffer.from('LXP/v1/receipt\0'), encodeKernelReceipt(receipt, false)])
    );
    receipt.sequencerSignature = ed25519.sign(digest, seed);
    entry.signedReceipt = entry.receiptJson.receipt = bytesToHex(encodeKernelReceipt(receipt));
    entry.receiptDigest = bytesToHex(digest);
    if (changeAnswer) changeAnswer(entry);
    entry.expected = { verifier: 'rejects' };
    cases.push(entry);
  }
  negative('V3-key-not-pinned', 'Signed receipt uses an ungoverned key id', (receipt, entry) => {
    receipt.contextHash.fill(0x6c);
    entry.receiptJson.keyId = bytesToHex(receipt.contextHash);
  });
  negative(
    'V3-height-outside-key-range',
    'Signed height exceeds the governed pin range',
    (receipt, entry) => {
      const height = BigInt(entry.pinnedKey.lastHeight) + 1n;
      new DataView(receipt.batchId.buffer).setBigUint64(24, height, false);
      entry.receiptJson.height = `0x${height.toString(16)}`;
    }
  );
  negative(
    'V3-activity-id-not-raw-hash',
    'Signed receipt belongs to another raw transaction',
    undefined,
    (entry) => {
      entry.rawTransaction = cases[0].rawTransaction;
    }
  );
  negative(
    'V3-seq-hash-mismatch',
    'Valid signature covers an inconsistent resulting sequence hash',
    (receipt) => {
      receipt.resultingStateRoot[0] ^= 1;
    }
  );
  negative(
    'V3-tampered-log',
    'JSON log data differs from the signed effect',
    undefined,
    (entry) => {
      const data = hexToBytes(entry.receiptJson.logs[0].data);
      data[data.length - 1] ^= 1;
      entry.receiptJson.logs[0].data = bytesToHex(data);
    }
  );
  negative(
    'V3-non-canonical-bytes',
    'Signature presence flag is not canonical',
    undefined,
    (entry) => {
      const bytes = hexToBytes(entry.signedReceipt);
      bytes[bytes.length - 69] = 2;
      entry.signedReceipt = entry.receiptJson.receipt = bytesToHex(bytes);
    }
  );
  const corpus = {
    name: 'kernel-receipt-v1',
    provenance: {
      generator: 'sdk/platform/conformance/fixtures/generate_kernel_receipt_fixture.mjs',
      command: 'node sdk/platform/conformance/fixtures/generate_kernel_receipt_fixture.mjs',
      encoder: 'sdk/wallet/src/receipt.ts#encodeKernelReceipt',
      sourceGenerator: source.provenance.generator,
      sourceFixture: 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json',
    },
    chainId: source.chainId,
    moduleId: source.moduleId,
    feeDenom: source.feeDenom,
    cases,
  };
  await writeFile(
    new URL('./kernel-receipt-v1.json', import.meta.url),
    `${JSON.stringify(corpus, null, 2)}\n`
  );
} finally {
  await server.close();
}
