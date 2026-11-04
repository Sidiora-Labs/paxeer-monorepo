@external("layerx_v1", "storage_read")
declare function storageRead(key: i32, keyLength: i32, output: i32, capacity: i32): i32;

export function read(key: i32, keyLength: i32, output: i32, capacity: i32): i32 {
  return storageRead(key, keyLength, output, capacity);
}
