/**
 * Task 22.31.375, audit 22.30.129 finding 003.
 * Proving tests: repair003StatusTaxonomy, repair003Ed25519HostEvidence,
 * repair003Secp256k1HostEvidence, repair003MalformedHostArguments.
 * Baseline was not run: the ordered gate rules prohibit a pre-change verify.
 * Before the repair, the -6 classification, ABI and diagnostic assertions fail
 * by inspection; no pre-change execution or runtime qualification is claimed.
 * Wiring request (domain 13): programs/crates/layerx-programs-runtime/tests
 * must execute this guest's layerx_main selectors 0..3 with the real ABI-v2
 * signature host and compare terminal statuses -6, -6, -2, 0 with equivalent
 * programs/sdk/rust and programs/sdk/c guests and their execution evidence.
 * The declared asc gate compiles these assertions without executing the guest.
 */

import {
  OK,
  ERR_INVALID,
  ERR_BOUNDS,
  ERR_EVIDENCE,
  ERR_NULL_ARGUMENT,
  ERR_OVERFLOW,
  STATUS_VERIFY_FAILED,
  STATUS_WEB_ABSENT,
  abiStatus,
  isHostRefusal,
  isRefusal,
  statusName,
  acceptCallInput,
  callInputBytes,
  reserveCallInput,
  verifyEd25519V2,
  verifySecp256k1V2
} from "../assembly/index";

function hexBytes(hex: string): StaticArray<u8> {
  assert(hex.length % 2 == 0);
  const bytes = new StaticArray<u8>(hex.length / 2);
  const alphabet = "0123456789abcdef";
  for (let i = 0; i < bytes.length; i++) {
    const high = alphabet.indexOf(hex.charAt(i * 2));
    const low = alphabet.indexOf(hex.charAt(i * 2 + 1));
    assert(high >= 0 && low >= 0);
    bytes[i] = <u8>((high << 4) | low);
  }
  return bytes;
}

function assertVerificationEvidence(status: i32): i32 {
  assert(status == STATUS_VERIFY_FAILED);
  assert(status == -6);
  assert(isRefusal(status));
  assert(isHostRefusal(status));
  assert(statusName(status) == "LXP_PROGRAM_ERR_VERIFY_FAILED");
  const terminal = abiStatus(status);
  assert(terminal == status);
  assert(terminal != ERR_INVALID);
  return terminal;
}

export function repair003StatusTaxonomy(): i32 {
  assert(STATUS_VERIFY_FAILED == -6);
  for (let status = ERR_EVIDENCE; status < 0; status++) {
    assert(isHostRefusal(status));
    assert(abiStatus(status) == status);
  }
  assertVerificationEvidence(STATUS_VERIFY_FAILED);
  assert(!isRefusal(OK));
  assert(!isHostRefusal(OK));
  assert(!isHostRefusal(7));
  assert(abiStatus(7) == 7);
  assert(!isHostRefusal(STATUS_WEB_ABSENT));
  assert(abiStatus(STATUS_WEB_ABSENT) == ERR_INVALID);
  assert(!isHostRefusal(-15));
  assert(abiStatus(-15) == ERR_INVALID);
  assert(!isHostRefusal(ERR_NULL_ARGUMENT));
  assert(abiStatus(ERR_NULL_ARGUMENT) == ERR_INVALID);
  assert(!isHostRefusal(ERR_OVERFLOW));
  assert(abiStatus(ERR_OVERFLOW) == ERR_BOUNDS);
  assert(!isHostRefusal(i32.MIN_VALUE));
  assert(abiStatus(i32.MIN_VALUE) == ERR_INVALID);
  assert(statusName(ERR_INVALID) == "LXP_PROGRAM_ERR_INVALID");
  assert(statusName(-15) == "LXP_PROGRAM_ERR_UNKNOWN");
  return OK;
}

export function repair003Ed25519HostEvidence(): i32 {
  // RFC 8032 section 7.1, TEST 1; also used by runtime signature_vectors.rs.
  const key = hexBytes("d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a");
  const signature = hexBytes("e5564300c360ac729086e2cc806e828a84877f1eb8e5d974d873e065224901555fb8821590a33bacc61e39701cf9b46bd25bf5f0595bbe24655141438e7a100b");
  const valid = verifyEd25519V2(new StaticArray<u8>(0), key, signature);
  assert(valid == OK);
  assert(abiStatus(valid) == OK);
  // A different message preserves the valid key/signature encodings.
  const invalid = verifyEd25519V2(new StaticArray<u8>(1), key, signature);
  return assertVerificationEvidence(invalid);
}

export function repair003Secp256k1HostEvidence(): i32 {
  // Wycheproof tcId 3, shared with runtime signature_vectors.rs.
  const digest = hexBytes("bb5a52f42f9c9261ed4361f59422a1e30036e7c32b270c8807a419feca605023");
  const key = hexBytes("02782c8ed17e3b2a783b5464f33b09652a71c678e05ec51e84e2bcfc663a3de963");
  const signature = hexBytes("d035ee1f17fdb0b2681b163e33c359932659990af77dca632012b30b27a057b31939d9f3b2858bc13e3474cb50e6a82be44faa71940f876c1cba4c3e989202b6");
  assert(verifySecp256k1V2(digest, key, signature) == OK);
  digest[0] ^= 1;
  return assertVerificationEvidence(verifySecp256k1V2(digest, key, signature));
}

export function repair003MalformedHostArguments(): i32 {
  const digest = new StaticArray<u8>(32);
  const key = new StaticArray<u8>(33);
  const signature = hexBytes("d035ee1f17fdb0b2681b163e33c359932659990af77dca632012b30b27a057b31939d9f3b2858bc13e3474cb50e6a82be44faa71940f876c1cba4c3e989202b6");
  // SDK lengths are valid; the real host refuses the malformed SEC1 key.
  const malformed = verifySecp256k1V2(digest, key, signature);
  assert(malformed == ERR_INVALID);
  assert(malformed != STATUS_VERIFY_FAILED);
  assert(isHostRefusal(malformed));
  assert(statusName(malformed) == "LXP_PROGRAM_ERR_INVALID");
  assert(abiStatus(malformed) == ERR_INVALID);
  const bounded = verifyEd25519V2(digest, new StaticArray<u8>(31), signature);
  assert(bounded == ERR_BOUNDS);
  assert(abiStatus(bounded) == ERR_BOUNDS);
  return abiStatus(malformed);
}

export function layerx_main(selector: i64): i64 {
  if (selector == 0) return <i64>repair003Ed25519HostEvidence();
  if (selector == 1) return <i64>repair003Secp256k1HostEvidence();
  if (selector == 2) return <i64>repair003MalformedHostArguments();
  if (selector == 3) return <i64>repair003StatusTaxonomy();
  return <i64>ERR_INVALID;
}

export function layerx_reserve(length: i32): i32 {
  return reserveCallInput(length);
}

export function layerx_call(inputPointer: i32, inputLength: i32): i32 {
  const admitted = acceptCallInput(inputPointer, inputLength);
  if (admitted < 0) return abiStatus(admitted);
  if (admitted != 1) return ERR_INVALID;
  return <i32>layerx_main(<i64>callInputBytes(admitted)[0]);
}
