import {
  calculateExistenceRoot,
  ics23,
  iavlSpec,
  tendermintSpec,
  verifyMembership,
  verifyNonMembership,
} from '@confio/ics23';
import { hexToBytes, utf8ToBytes } from '@noble/hashes/utils.js';

export type ProofOp = {
  type: string;
  key: Uint8Array | string;
  data: Uint8Array | string;
};

export type StorageProofInput = {
  proofOps: readonly ProofOp[] | { ops: readonly ProofOp[] };
  appHash: Uint8Array;
  store: string;
  key: Uint8Array;
  value: Uint8Array;
};

export type StorageAbsenceInput = Omit<StorageProofInput, 'value'>;

export class ProofError extends Error {
  constructor(
    public readonly op: string,
    message: string,
    cause?: unknown
  ) {
    super(`${op}: ${message}`, { cause });
    this.name = 'ProofError';
  }
}

function opBytes(value: Uint8Array | string): Uint8Array {
  if (value instanceof Uint8Array) return value;
  if (value.startsWith('0x')) return hexToBytes(value.slice(2));
  return Uint8Array.from(atob(value), (character) => character.charCodeAt(0));
}

export function verifyStorageProof({
  proofOps,
  appHash,
  store,
  key,
  value,
}: StorageProofInput): true {
  const ops = 'ops' in proofOps ? proofOps.ops : proofOps;
  const stages = [
    { type: 'ics23:iavl', spec: iavlSpec },
    { type: 'ics23:simple', spec: tendermintSpec },
  ];
  if (ops.length !== 2) {
    const failing = ops.length < 2 ? stages[ops.length]!.type : ops[2]!.type;
    throw new ProofError(failing, 'Expected exactly the IAVL and multistore proof ops');
  }
  let expectedKey = key;
  let expectedValue = value;
  for (const [index, stage] of stages.entries()) {
    const op = ops[index]!;
    try {
      if (op.type !== stage.type) throw new Error(`Unexpected op ${op.type}`);
      const opKey = opBytes(op.key);
      if (
        opKey.length !== expectedKey.length ||
        !opKey.every((byte, i) => byte === expectedKey[i])
      ) {
        throw new Error('Op key does not match the requested key');
      }
      const proof = ics23.CommitmentProof.decode(opBytes(op.data));
      if (!proof.exist || proof.nonexist || proof.batch || proof.compressed) {
        throw new Error('Expected an existence proof');
      }
      const root = index === 0 ? calculateExistenceRoot(proof.exist) : appHash;
      if (
        root.length !== 32 ||
        !verifyMembership(proof, stage.spec, root, expectedKey, expectedValue)
      ) {
        throw new Error('Membership verification failed');
      }
      expectedKey = utf8ToBytes(store);
      expectedValue = root;
    } catch (error) {
      throw new ProofError(
        stage.type,
        error instanceof Error ? error.message : String(error),
        error
      );
    }
  }
  return true;
}

export function verifyStorageAbsence({
  proofOps,
  appHash,
  store,
  key,
}: StorageAbsenceInput): true {
  const ops = 'ops' in proofOps ? proofOps.ops : proofOps;
  const stages = ['ics23:iavl', 'ics23:simple'];
  if (ops.length !== 2) {
    const failing = ops.length < 2 ? stages[ops.length]! : ops[2]!.type;
    throw new ProofError(failing, 'Expected exactly the IAVL and multistore proof ops');
  }
  let expectedKey = key;
  let storeRoot: Uint8Array = new Uint8Array();
  for (const [index, stage] of stages.entries()) {
    const op = ops[index]!;
    try {
      if (op.type !== stage) throw new Error(`Unexpected op ${op.type}`);
      const opKey = opBytes(op.key);
      if (
        opKey.length !== expectedKey.length ||
        !opKey.every((byte, i) => byte === expectedKey[i])
      ) {
        throw new Error('Op key does not match the requested key');
      }
      const proof = ics23.CommitmentProof.decode(opBytes(op.data));
      if (index === 0) {
        if (!proof.nonexist || proof.exist || proof.batch || proof.compressed) {
          throw new Error('Expected a non-existence proof');
        }
        const absentKey = proof.nonexist.key;
        if (
          !absentKey ||
          absentKey.length !== key.length ||
          !absentKey.every((byte, i) => byte === key[i])
        ) {
          throw new Error('Non-existence proof key does not match the requested key');
        }
        const neighbour = proof.nonexist.left ?? proof.nonexist.right;
        if (!neighbour) throw new Error('Non-existence proof has no neighbour');
        storeRoot = calculateExistenceRoot(neighbour);
        if (
          storeRoot.length !== 32 ||
          !verifyNonMembership(proof, iavlSpec, storeRoot, key)
        ) {
          throw new Error('Non-membership verification failed');
        }
      } else {
        if (!proof.exist || proof.nonexist || proof.batch || proof.compressed) {
          throw new Error('Expected an existence proof');
        }
        if (
          appHash.length !== 32 ||
          !verifyMembership(proof, tendermintSpec, appHash, expectedKey, storeRoot)
        ) {
          throw new Error('Membership verification failed');
        }
      }
      expectedKey = utf8ToBytes(store);
    } catch (error) {
      throw new ProofError(
        stage,
        error instanceof Error ? error.message : String(error),
        error
      );
    }
  }
  return true;
}
