"use strict";

const NATIVE_SURFACE_RUNTIME =
  "0x363d3d373360601b36523d3d601436013d3d6110175af13d600060003e6024573d6000fd5b3d6000f3";

function isNativeSurface(code) {
  return String(code).toLowerCase() === NATIVE_SURFACE_RUNTIME;
}

async function assertNotNativeSurface(provider, address, key) {
  if (isNativeSurface(await provider.getCode(address))) {
    throw new Error(`Native surface ${key}: upgrade through governance MsgUpdateParams`);
  }
}

module.exports = { NATIVE_SURFACE_RUNTIME, isNativeSurface, assertNotNativeSurface };
