import { reserveCallInput, acceptCallInput, callInputRegion } from "../../assemblyscript/assembly/entry";
import { verifyEd25519V2, verifySecp256k1V2 } from "../../assemblyscript/assembly/versioned";

export function layerx_reserve(length: i32): i32 {
  return reserveCallInput(length);
}

export function layerx_main(selector: i64): i64 {
  return 0;
}

export function layerx_call(pointer: i32, length: i32): i32 {
  const admitted = acceptCallInput(pointer, length);
  if (admitted < 0) return admitted;
  const input = callInputRegion();
  if (length < 3 || length != 3 + input[1] + input[2] + 64) return -2;
  const message = new StaticArray<u8>(input[2]);
  const key = new StaticArray<u8>(input[1]);
  const signature = new StaticArray<u8>(64);
  for (let i = 0; i < message.length; ++i) message[i] = input[3 + i];
  for (let i = 0; i < key.length; ++i) key[i] = input[3 + message.length + i];
  for (let i = 0; i < 64; ++i) signature[i] = input[3 + message.length + key.length + i];
  if (input[0] == 1) return verifyEd25519V2(message, key, signature);
  if (input[0] == 2) return verifySecp256k1V2(message, key, signature);
  return -2;
}
