import { secp256k1 } from "@noble/curves/secp256k1.js";
import {
  eip1559SigningHash,
  keccak256,
  recoverEip1559Sender,
  verifySequencerReceipt,
  type Eip1559Transaction,
} from "@sidiora/layerx-sdk";
import {
  LAYERX_LEDGER_ADDRESS,
  type PortableNodeReceiptV2,
  type PaymentPayload,
  type PaymentRequired,
  type PaymentRequirements,
  type PortableReceiptV2,
} from "@sidiora/layerx-seller-middleware";

export interface ReceiptKeyEntry {
  readonly keyId: Uint8Array;
  readonly firstHeight: bigint;
  readonly lastHeight: bigint;
  readonly publicKey: Uint8Array;
}

export class ConformanceSequencer {
  readonly #privateKey: CryptoKey;
  readonly #payerKey: Uint8Array;
  readonly #keyId: Uint8Array;

  readonly #publicKey: Uint8Array;

  private constructor(privateKey: CryptoKey, publicKey: Uint8Array, payerKey: Uint8Array) {
    this.#privateKey = privateKey;
    this.#publicKey = publicKey.slice();
    this.#payerKey = payerKey;
    this.#keyId = keccak256(publicKey);
  }

  public get publicKey(): Uint8Array {
    return this.#publicKey.slice();
  }

  public static async generate(): Promise<ConformanceSequencer> {
    const pair = (await globalThis.crypto.subtle.generateKey(
      { name: "Ed25519" },
      true,
      ["sign", "verify"],
    )) as CryptoKeyPair;
    const raw = new Uint8Array(await globalThis.crypto.subtle.exportKey("raw", pair.publicKey));
    return new ConformanceSequencer(pair.privateKey, raw, secp256k1.utils.randomSecretKey());
  }

  public receiptKey(): ReceiptKeyEntry {
    return {
      keyId: this.#keyId.slice(),
      firstHeight: 1n,
      lastHeight: 1_000n,
      publicKey: this.#publicKey.slice(),
    };
  }

  public async sign(digest: Uint8Array): Promise<Uint8Array> {
    const signature = await globalThis.crypto.subtle.sign(
      { name: "Ed25519" },
      this.#privateKey,
      arrayBuffer(digest),
    );
    return new Uint8Array(signature);
  }

  public signTransaction(transaction: Eip1559Transaction): Uint8Array {
    const signature = secp256k1.sign(eip1559SigningHash(transaction), this.#payerKey, {
      prehash: false,
      format: "recovered",
    });
    const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
    return concatenate(Uint8Array.of(2), rlp([
      integer(transaction.chainId), integer(transaction.nonce),
      integer(transaction.maxPriorityFeePerGas), integer(transaction.maxFeePerGas),
      integer(transaction.gasLimit), transaction.to, integer(transaction.value), transaction.data,
      transaction.accessList.map((entry) => [entry.address, [...entry.storageKeys]]),
      integer(BigInt(signature[0]!)), integer(parsed.r), integer(parsed.s),
    ]));
  }
}

export interface ReceiptFacts {
  readonly asset: Uint8Array;
  readonly payTo: Uint8Array;
  readonly amount: bigint;
  readonly purposeHash?: string;
  readonly from: Uint8Array;
  readonly batchId: Uint8Array;
  readonly previousStateRoot: Uint8Array;
  readonly resultingStateRoot: Uint8Array;
}

export interface SignedReceipt extends PortableReceiptV2 {
  readonly txHash: string;
  readonly receiptJson: PortableNodeReceiptV2["receiptJson"];
}

export async function buildSignedReceipt(
  sequencer: ConformanceSequencer,
  facts: ReceiptFacts,
): Promise<SignedReceipt> {
  for (const word of [facts.asset, facts.payTo, facts.from, facts.previousStateRoot, facts.resultingStateRoot]) {
    if (word.length !== 32) throw new Error("receipt facts must contain 32-byte words");
  }
  bigEndian(facts.amount, 16);
  if (facts.purposeHash !== undefined
    && (!/^[0-9a-f]{64}$/u.test(facts.purposeHash) || /^0+$/u.test(facts.purposeHash))) {
    throw new Error("receipt purpose hash must be a nonzero 32-byte lowercase hex string");
  }
  const reference = facts.purposeHash === undefined
    ? paymentReference(facts.payTo, facts.asset, facts.amount)
    : fromHex(`0x${facts.purposeHash}`);
  const ledger = fromHex(LAYERX_LEDGER_ADDRESS);
  const rawTransaction = sequencer.signTransaction({
    chainId: 125n,
    nonce: 0n,
    maxPriorityFeePerGas: 1n,
    maxFeePerGas: 2n,
    gasLimit: 200_000n,
    to: ledger,
    value: 0n,
    data: concatenate(
      keccak256(new TextEncoder().encode("payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)")).slice(0, 4),
      facts.from, facts.payTo, facts.asset, bigEndian(facts.amount, 32), bigEndian(1n, 32), reference,
    ),
    accessList: [],
  });
  const txHash = keccak256(rawTransaction);
  const logData = concatenate(facts.asset, bigEndian(facts.amount, 32));
  const logBody = concatenate(
    ledger, Uint8Array.of(4),
    keccak256(new TextEncoder().encode("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)")),
    reference, facts.from, facts.payTo, await sha256(logData),
  );
  const summary = new ByteWriter();
  summary.u64(50_000n);
  summary.u64(50_000n);
  summary.u64(2n);
  summary.u32(0);
  summary.raw(new Uint8Array(40));
  summary.raw(await sha256(new Uint8Array()));
  summary.u32(0);
  summary.u32(1);
  summary.u8(4);
  summary.raw(new TextEncoder().encode("uhpx"));

  const key = sequencer.receiptKey();
  const writer = new ByteWriter();
  writer.u16(2);
  writer.u16(0x5201);
  writer.u16(2);
  writer.bounded(txHash);
  writer.u64(1n);
  writer.bounded(facts.previousStateRoot);
  writer.bounded(facts.resultingStateRoot);
  writer.bounded(await sha256(new Uint8Array()));
  writer.u32(0);
  writer.u32(2);
  writeEffect(writer, 0, 1, summary.finish());
  writeEffect(writer, 1, 3, logBody);
  writer.u128(100_000n);
  writer.bounded(bigEndian(7n, 32));
  writer.u16(12);
  writer.u32(1);
  writer.u32(0);
  writer.u8(2);
  writer.bounded(await sha256(new TextEncoder().encode("uhpx")));
  writer.u128(0n);
  writer.bounded(concatenate(new Uint8Array(12), fromHex(recoverEip1559Sender(rawTransaction))));
  writer.u128(1_000_000n);
  writer.u128(900_000n);
  writer.u64(0n);
  writer.bounded(concatenate(new Uint8Array(12), ledger));
  writer.u128(0n);
  writer.u128(0n);
  writer.bounded(new Uint8Array(32));
  writer.bounded(new Uint8Array(32));
  writer.bounded(key.keyId);
  writer.u64(1_726_000_000_000n);
  const prefix = writer.finish();
  const digest = await sha256(
    new TextEncoder().encode("LXP/v1/receipt\0"), prefix, Uint8Array.of(0),
  );
  const signature = await sequencer.sign(digest);
  const signed = new ByteWriter();
  signed.raw(prefix);
  signed.u8(1);
  signed.bounded64(signature);
  const signedReceipt = signed.finish();
  const verified = await verifySequencerReceipt(signedReceipt, rawTransaction, [{
    keyId: key.keyId, publicKey: key.publicKey,
    first_height: key.firstHeight, last_height: key.lastHeight,
  }]);
  const { receipt } = verified;
  const body = receipt.effects[0]!.body;
  const view = new DataView(arrayBuffer(body));
  const quantity = (value: bigint | number): string => `0x${value.toString(16)}`;
  return {
    format: "layerx-receipt-proof-v2",
    verificationLevel: "receipted",
    rawTransaction: hex(rawTransaction),
    signedReceipt: hex(signedReceipt),
    receiptDigest: hex(verified.receiptDigest),
    receiptKey: {
      keyId: hex(key.keyId),
      firstHeight: `0x${key.firstHeight.toString(16)}`,
      lastHeight: `0x${key.lastHeight.toString(16)}`,
      publicKey: hex(key.publicKey),
    },
    txHash: hex(txHash),
    receiptJson: {
      rung: "receipted", txHash: hex(receipt.activityId), nonce: quantity(receipt.fromSequence),
      seq: quantity(receipt.globalSequence), height: quantity(verified.height), index: quantity(view.getUint32(24, false)),
      timestamp: quantity(receipt.timestamp), status: quantity(receipt.resultCode === 0 ? 1 : 0),
      returnData: "0x", logs: [{ address: hex(ledger),
        topics: [hex(logBody.subarray(21, 53)), hex(reference), hex(facts.from), hex(facts.payTo)],
        data: hex(logData), logIndex: "0x0" }],
      gasUsed: quantity(view.getBigUint64(0, false)), cumulativeGasUsed: quantity(view.getBigUint64(8, false)),
      effectiveGasPrice: quantity(view.getBigUint64(16, false)), paymentDenom: "uhpx",
      feeDenom: new TextDecoder().decode(body.subarray(109)), feeAmount: quantity(receipt.feeCharged),
      sponsor: hex(body.subarray(48, 68)), from: hex(receipt.from.subarray(12)), to: hex(receipt.to.subarray(12)),
      contractAddress: hex(body.subarray(28, 48)), vmError: "", keyId: hex(receipt.contextHash),
      signingKey: hex(verified.receiptKey.publicKey), receipt: hex(signedReceipt),
    },
  };
}

export interface OfferFixture {
  readonly requirements: PaymentRequirements;
  readonly paymentRequired: PaymentRequired;
}

export function offerFixture(payTo: Uint8Array, asset: Uint8Array, amount: bigint): OfferFixture {
  if (payTo.length !== 32 || asset.length !== 32) throw new Error("offer identifiers must be 32 bytes");
  bigEndian(amount, 16);
  const requirements: PaymentRequirements = {
    scheme: "exact",
    network: "eip155:125",
    amount: amount.toString(),
    asset: toHex(asset),
    payTo: toHex(payTo),
    maxTimeoutSeconds: 120,
    extra: { layerx: { commitment: "receipted", purposeHash: toHex(paymentReference(payTo, asset, amount)) } },
  };
  const paymentRequired: PaymentRequired = {
    x402Version: 2,
    resource: {
      url: "https://paid.example/paid",
      description: "conformance resource",
      mimeType: "application/json",
    },
    accepts: [requirements],
    extensions: {},
  };
  return { requirements, paymentRequired };
}

export function buyerPayload(offer: OfferFixture, receipt: SignedReceipt, key: string): PaymentPayload {
  return {
    x402Version: 2,
    resource: offer.paymentRequired.resource,
    payload: {
      format: receipt.format,
      verificationLevel: receipt.verificationLevel,
      rawTransaction: receipt.rawTransaction,
      signedReceipt: receipt.signedReceipt,
      receiptDigest: receipt.receiptDigest,
      receiptKey: { ...receipt.receiptKey },
    },
    accepted: offer.requirements,
    extensions: { layerx: { info: { idempotencyKey: key }, schema: { type: "object" } } },
  };
}

export function fixedBytes(seed: number): Uint8Array {
  return new Uint8Array(32).fill(seed & 0xff);
}

function paymentReference(payTo: Uint8Array, asset: Uint8Array, amount: bigint): Uint8Array {
  return keccak256(concatenate(payTo, asset, bigEndian(amount, 32)));
}

function writeEffect(writer: ByteWriter, ordinal: number, kind: number, body: Uint8Array): void {
  writer.u16(12);
  writer.u16(ordinal);
  writer.u16(0);
  writer.u8(kind);
  writer.u8(0);
  writer.bounded(new Uint8Array(32));
  writer.u32(body.length);
  writer.raw(body);
}

class ByteWriter {
  #chunks: Uint8Array[] = [];

  public u8(value: number): void {
    this.#chunks.push(new Uint8Array([value & 0xff]));
  }

  public u16(value: number): void {
    this.#chunks.push(bigEndian(BigInt(value), 2));
  }

  public u32(value: number): void {
    this.#chunks.push(bigEndian(BigInt(value), 4));
  }

  public u64(value: bigint): void {
    this.#chunks.push(bigEndian(value, 8));
  }

  public u128(value: bigint): void {
    this.#chunks.push(bigEndian(value, 16));
  }

  public bounded(value: Uint8Array): void {
    if (value.length !== 32) {
      throw new Error(`bounded field must be 32 bytes, got ${value.length}`);
    }
    this.u32(32);
    this.#chunks.push(value);
  }

  public bounded64(value: Uint8Array): void {
    if (value.length !== 64) {
      throw new Error(`signature field must be 64 bytes, got ${value.length}`);
    }
    this.u32(64);
    this.#chunks.push(value);
  }

  public raw(value: Uint8Array): void {
    this.#chunks.push(value);
  }

  public finish(): Uint8Array {
    return concatenate(...this.#chunks);
  }
}

type RlpValue = Uint8Array | readonly RlpValue[];

function integer(value: bigint): Uint8Array {
  const bytes = bigEndian(value, 32);
  const first = bytes.findIndex((byte) => byte !== 0);
  return first === -1 ? new Uint8Array() : bytes.slice(first);
}

function rlp(value: RlpValue): Uint8Array {
  const list = !(value instanceof Uint8Array);
  const payload = value instanceof Uint8Array ? value : concatenate(...value.map(rlp));
  if (!list && payload.length === 1 && payload[0]! < 128) return payload;
  const prefix = list ? 192 : 128;
  if (payload.length < 56) return concatenate(Uint8Array.of(prefix + payload.length), payload);
  const length = integer(BigInt(payload.length));
  return concatenate(Uint8Array.of(prefix + 55 + length.length), length, payload);
}

function bigEndian(value: bigint, length: number): Uint8Array {
  if (value < 0n) {
    throw new Error("cannot encode a negative integer");
  }
  const result = new Uint8Array(length);
  let remaining = value;
  for (let index = length - 1; index >= 0; index -= 1) {
    result[index] = Number(remaining & 0xffn);
    remaining >>= 8n;
  }
  if (remaining !== 0n) {
    throw new Error(`integer ${value} overflows ${length} bytes`);
  }
  return result;
}

function arrayBuffer(value: Uint8Array): ArrayBuffer {
  const copy = new ArrayBuffer(value.length);
  new Uint8Array(copy).set(value);
  return copy;
}

function concatenate(...values: readonly Uint8Array[]): Uint8Array {
  const result = new Uint8Array(values.reduce((total, value) => total + value.length, 0));
  let offset = 0;
  for (const value of values) {
    result.set(value, offset);
    offset += value.length;
  }
  return result;
}

async function sha256(...values: readonly Uint8Array[]): Promise<Uint8Array> {
  const digest = await globalThis.crypto.subtle.digest("SHA-256", arrayBuffer(concatenate(...values)));
  return new Uint8Array(digest);
}

function fromHex(value: string): Uint8Array {
  return Uint8Array.from(value.slice(2).match(/../gu)!, (byte) => Number.parseInt(byte, 16));
}

function hex(value: Uint8Array): string {
  return `0x${toHex(value)}`;
}

export function encodeBase64(value: Uint8Array): string {
  let binary = "";
  for (let offset = 0; offset < value.length; offset += 0x8000) {
    binary += String.fromCharCode(...value.subarray(offset, Math.min(offset + 0x8000, value.length)));
  }
  return btoa(binary);
}

export function toHex(value: Uint8Array): string {
  return Array.from(value, (byte) => byte.toString(16).padStart(2, "0")).join("");
}
