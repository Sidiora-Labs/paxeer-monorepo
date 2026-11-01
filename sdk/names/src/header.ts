import { ed25519 } from '@noble/curves/ed25519.js';
import { sha256 } from '@noble/hashes/sha2.js';
import { bytesToHex, concatBytes, hexToBytes, utf8ToBytes } from '@noble/hashes/utils.js';
import type { HeaderSource } from './resolve.js';

type Integer = bigint | number | string;
type PartSetHeader = { total: Integer; hash: string };
type BlockId = { hash: string; part_set_header: PartSetHeader };

export type TendermintHeader = {
  version: { block: Integer; app: Integer };
  chain_id: string;
  height: Integer;
  time: string;
  last_block_id: { hash: string; parts: PartSetHeader };
  last_commit_hash: string;
  data_hash: string;
  validators_hash: string;
  next_validators_hash: string;
  consensus_hash: string;
  app_hash: string;
  last_results_hash: string;
  evidence_hash: string;
  proposer_address: string;
};

export type CommitSignature = {
  block_id_flag: number;
  validator_address: string;
  timestamp: string;
  signature: string | null;
};

export type TendermintCommit = {
  height: Integer;
  round: Integer;
  signatures: readonly CommitSignature[];
} & ({ block_id: BlockId; blockId?: never } | { blockId: BlockId; block_id?: never });

export type Validator = { pub_key: string; voting_power: Integer };

export class HeaderError extends Error {
  constructor(message: string, cause?: unknown) {
    super(message, { cause });
    this.name = 'HeaderError';
  }
}

const INT64_MAX = (1n << 63n) - 1n;
const UINT64_MAX = (1n << 64n) - 1n;

function integer(value: unknown, max = INT64_MAX): bigint {
  if (
    !(
      typeof value === 'bigint' ||
      (typeof value === 'number' && Number.isSafeInteger(value)) ||
      (typeof value === 'string' && /^(0|[1-9][0-9]*)$/.test(value))
    )
  ) {
    throw new HeaderError('Expected an exact nonnegative integer');
  }
  const result = BigInt(value);
  if (result < 0n || result > max) throw new HeaderError('Integer outside consensus bounds');
  return result;
}

function hex(value: string, size?: number): Uint8Array {
  if (typeof value !== 'string' || !/^(?:[0-9a-fA-F]{2})*$/.test(value)) {
    throw new HeaderError('Expected hexadecimal consensus bytes');
  }
  const result = hexToBytes(value);
  if (size !== undefined && result.length !== size) {
    throw new HeaderError(`Expected ${size} consensus bytes`);
  }
  return result;
}

function base64(value: string, size: number): Uint8Array {
  if (
    typeof value !== 'string' ||
    !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(value)
  ) {
    throw new HeaderError('Expected base64 consensus bytes');
  }
  const result = Uint8Array.from(atob(value), (character) => character.charCodeAt(0));
  if (result.length !== size) throw new HeaderError(`Expected ${size} consensus bytes`);
  return result;
}

function varint(value: bigint): Uint8Array {
  let remaining = BigInt.asUintN(64, value);
  const bytes: number[] = [];
  while (remaining >= 128n) {
    bytes.push(Number(remaining & 127n) | 128);
    remaining >>= 7n;
  }
  bytes.push(Number(remaining));
  return Uint8Array.from(bytes);
}

function varintField(field: number, value: bigint): Uint8Array {
  return value === 0n ? new Uint8Array() : concatBytes(varint(BigInt(field << 3)), varint(value));
}

function fixed64Field(field: number, value: bigint): Uint8Array {
  if (value === 0n) return new Uint8Array();
  const bytes = new Uint8Array(8);
  new DataView(bytes.buffer).setBigUint64(0, value, true);
  return concatBytes(varint(BigInt((field << 3) | 1)), bytes);
}

function bytesField(field: number, value: Uint8Array, message = false): Uint8Array {
  if (value.length === 0 && !message) return new Uint8Array();
  return concatBytes(varint(BigInt((field << 3) | 2)), varint(BigInt(value.length)), value);
}

function timestamp(value: string): Uint8Array {
  if (typeof value !== 'string') throw new HeaderError('Expected an RFC3339 timestamp');
  const match = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})$/.exec(
    value
  );
  if (!match) throw new HeaderError('Expected an RFC3339 nanosecond timestamp');
  const milliseconds = Date.parse(`${match[1]}${match[3]}`);
  if (!Number.isFinite(milliseconds)) throw new HeaderError('Invalid timestamp');
  const seconds = BigInt(milliseconds / 1000);
  if (seconds < -62135596800n || seconds > 253402300799n) {
    throw new HeaderError('Timestamp outside protobuf bounds');
  }
  const nanos = BigInt((match[2] ?? '').padEnd(9, '0'));
  return concatBytes(varintField(1, seconds), varintField(2, nanos));
}

function blockIdBytes(block: BlockId): Uint8Array {
  const hash = hex(block.hash);
  const partsHash = hex(block.part_set_header.hash);
  const total = integer(block.part_set_header.total, 0xffffffffn);
  const nil = hash.length === 0 && partsHash.length === 0 && total === 0n;
  if (!nil && (hash.length !== 32 || partsHash.length !== 32 || total === 0n)) {
    throw new HeaderError('Block ID must be empty or complete');
  }
  return concatBytes(
    bytesField(1, hash),
    bytesField(2, concatBytes(varintField(1, total), bytesField(2, partsHash)), true)
  );
}

function commitBlockId(commit: TendermintCommit): BlockId {
  const block = commit.block_id ?? commit.blockId;
  if (block === undefined) throw new HeaderError('Missing commit block ID');
  return block;
}

function merkleRoot(fields: readonly Uint8Array[]): Uint8Array {
  if (fields.length === 0) return sha256(new Uint8Array());
  if (fields.length === 1) return sha256(concatBytes(Uint8Array.of(0), fields[0]!));
  let split = 1;
  while (split * 2 < fields.length) split *= 2;
  return sha256(
    concatBytes(
      Uint8Array.of(1),
      merkleRoot(fields.slice(0, split)),
      merkleRoot(fields.slice(split))
    )
  );
}

export function voteSignBytes({
  chainId,
  commit,
  signature,
}: {
  chainId: string;
  commit: TendermintCommit;
  signature: CommitSignature;
}): Uint8Array {
  if (signature.block_id_flag !== 2 && signature.block_id_flag !== 3) {
    throw new HeaderError('Only present precommits have sign bytes');
  }
  const block = commitBlockId(commit);
  const bytes = concatBytes(
    varintField(1, 2n),
    fixed64Field(2, integer(commit.height)),
    fixed64Field(3, integer(commit.round, 0x7fffffffn)),
    signature.block_id_flag === 2 ? bytesField(4, blockIdBytes(block), true) : new Uint8Array(),
    bytesField(5, timestamp(signature.timestamp), true),
    bytesField(6, utf8ToBytes(chainId))
  );
  return concatBytes(varint(BigInt(bytes.length)), bytes);
}

export function headerHash(header: TendermintHeader): Uint8Array {
  const fields = [
    concatBytes(
      varintField(1, integer(header.version.block, UINT64_MAX)),
      varintField(2, integer(header.version.app, UINT64_MAX))
    ),
    bytesField(1, utf8ToBytes(header.chain_id)),
    varintField(1, integer(header.height)),
    timestamp(header.time),
    blockIdBytes({ hash: header.last_block_id.hash, part_set_header: header.last_block_id.parts }),
    ...[
      header.last_commit_hash,
      header.data_hash,
      header.validators_hash,
      header.next_validators_hash,
      header.consensus_hash,
      header.app_hash,
      header.last_results_hash,
      header.evidence_hash,
      header.proposer_address,
    ].map((value) => bytesField(1, hex(value))),
  ];
  hex(header.validators_hash, 32);
  return merkleRoot(fields);
}

export function validatorSetHash(validators: readonly Validator[]): Uint8Array {
  return merkleRoot(
    validators.map((validator) =>
      concatBytes(
        bytesField(1, bytesField(1, base64(validator.pub_key, 32)), true),
        varintField(2, integer(validator.voting_power))
      )
    )
  );
}

export function verifyCommit({
  chainId,
  header,
  commit,
  validators,
}: {
  chainId: string;
  header: TendermintHeader;
  commit: TendermintCommit;
  validators: readonly Validator[];
}): { verified: true; signedPower: bigint; totalPower: bigint } {
  try {
    if (!chainId || chainId.length > 50 || header.chain_id !== chainId) {
      throw new HeaderError('Header chain ID does not match');
    }
    if (integer(header.height) === 0n || integer(header.height) !== integer(commit.height)) {
      throw new HeaderError('Commit height does not match the header');
    }
    integer(commit.round, 0x7fffffffn);
    const block = commitBlockId(commit);
    blockIdBytes(block);
    if (bytesToHex(hex(block.hash, 32)) !== bytesToHex(headerHash(header))) {
      throw new HeaderError('Commit block hash does not match the header');
    }
    if (validators.length === 0 || commit.signatures.length !== validators.length) {
      throw new HeaderError('Commit must contain one signature slot per validator');
    }
    const seen = new Set<string>();
    const entries = validators.map((validator) => {
      const key = base64(validator.pub_key, 32);
      const address = bytesToHex(sha256(key).slice(0, 20));
      const power = integer(validator.voting_power);
      if (power === 0n || seen.has(address))
        throw new HeaderError('Invalid or duplicate validator');
      seen.add(address);
      return { key, address, power };
    });
    const validatorsHash = validatorSetHash(validators);
    if (bytesToHex(validatorsHash) !== bytesToHex(hex(header.validators_hash, 32))) {
      throw new HeaderError('Header validator hash does not match the configured set');
    }
    let signedPower = 0n;
    let totalPower = 0n;
    for (const [index, validator] of entries.entries()) {
      totalPower += validator.power;
      const signature = commit.signatures[index]!;
      if (signature.block_id_flag === 1) {
        if (
          signature.validator_address !== '' ||
          (signature.signature !== null && signature.signature !== '') ||
          signature.timestamp !== '0001-01-01T00:00:00Z'
        ) {
          throw new HeaderError('Absent signature contains vote data');
        }
        continue;
      }
      if (bytesToHex(hex(signature.validator_address, 20)) !== validator.address) {
        throw new HeaderError('Signature validator address does not match its slot');
      }
      if (
        signature.signature === null ||
        !ed25519.verify(
          base64(signature.signature, 64),
          voteSignBytes({ chainId, commit, signature }),
          validator.key,
          { zip215: false }
        )
      ) {
        throw new HeaderError('Invalid commit signature');
      }
      if (signature.block_id_flag === 2) signedPower += validator.power;
    }
    if (signedPower * 3n <= totalPower * 2n) {
      throw new HeaderError('Commit requires more than two thirds of voting power');
    }
    return { verified: true, signedPower, totalPower };
  } catch (error) {
    if (error instanceof HeaderError) throw error;
    throw new HeaderError('Malformed header or commit', error);
  }
}

function record(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new HeaderError('Expected a consensus RPC object');
  }
  return value as Record<string, unknown>;
}

function text(value: unknown): string {
  if (typeof value !== 'string') throw new HeaderError('Expected a consensus RPC string');
  return value;
}

function partSet(value: unknown): PartSetHeader {
  const parts = record(value);
  return { total: integer(parts.total, 0xffffffffn), hash: text(parts.hash) };
}

function readHeader(value: unknown): TendermintHeader {
  const header = record(value);
  const version = record(header.version);
  const lastBlock = record(header.last_block_id);
  return {
    version: { block: integer(version.block, UINT64_MAX), app: integer(version.app, UINT64_MAX) },
    chain_id: text(header.chain_id),
    height: integer(header.height),
    time: text(header.time),
    last_block_id: { hash: text(lastBlock.hash), parts: partSet(lastBlock.parts) },
    last_commit_hash: text(header.last_commit_hash),
    data_hash: text(header.data_hash),
    validators_hash: text(header.validators_hash),
    next_validators_hash: text(header.next_validators_hash),
    consensus_hash: text(header.consensus_hash),
    app_hash: text(header.app_hash),
    last_results_hash: text(header.last_results_hash),
    evidence_hash: text(header.evidence_hash),
    proposer_address: text(header.proposer_address),
  };
}

function readCommit(value: unknown): TendermintCommit {
  const commit = record(value);
  const block = record(commit.block_id);
  if (!Array.isArray(commit.signatures)) throw new HeaderError('Missing commit signatures');
  return {
    height: integer(commit.height),
    round: integer(commit.round, 0x7fffffffn),
    block_id: { hash: text(block.hash), part_set_header: partSet(block.parts) },
    signatures: commit.signatures.map((value: unknown) => {
      const signature = record(value);
      return {
        block_id_flag: Number(integer(signature.block_id_flag, 3n)),
        validator_address: text(signature.validator_address),
        timestamp: text(signature.timestamp),
        signature: signature.signature === null ? null : text(signature.signature),
      };
    }),
  };
}

export function verifiedTendermintHeaders(
  url: string,
  { validators }: { validators: readonly Validator[] }
): HeaderSource {
  const trustedValidators = validators.map((validator) => ({ ...validator }));
  return {
    async appHashAt(height: bigint): Promise<Uint8Array> {
      try {
        if (integer(height) === 0n) throw new HeaderError('Header height must be positive');
        const endpoint = new URL(`${url.replace(/\/$/, '')}/commit`);
        endpoint.searchParams.set('height', height.toString());
        const response = await fetch(endpoint);
        if (!response.ok) throw new HeaderError(`Consensus RPC HTTP status ${response.status}`);
        const envelope = record(await response.json());
        if (envelope.error != null) throw new HeaderError('Consensus RPC returned an error');
        const signedHeader = record(record(envelope.result).signed_header);
        const header = readHeader(signedHeader.header);
        const commit = readCommit(signedHeader.commit);
        if (integer(header.height) !== height)
          throw new HeaderError('Consensus RPC returned the wrong header height');
        verifyCommit({ chainId: header.chain_id, header, commit, validators: trustedValidators });
        return hex(header.app_hash, 32);
      } catch (error) {
        if (error instanceof HeaderError) throw error;
        throw new HeaderError('Could not verify consensus header', error);
      }
    },
  };
}
