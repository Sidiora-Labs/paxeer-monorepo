import { sha256 } from "@noble/hashes/sha2.js";

export function encodeSequencerLogEffect(address: Uint8Array, topics: readonly Uint8Array[], data: Uint8Array): Uint8Array {
  if (address.length !== 20 || topics.length > 4 || topics.some(topic => topic.length !== 32)) {
    throw new TypeError("invalid sequencer log");
  }
  const encoded = new Uint8Array(53 + topics.length * 32);
  encoded.set(address);
  encoded[20] = topics.length;
  topics.forEach((topic, index) => encoded.set(topic, 21 + index * 32));
  encoded.set(sha256(data), encoded.length - 32);
  return encoded;
}
