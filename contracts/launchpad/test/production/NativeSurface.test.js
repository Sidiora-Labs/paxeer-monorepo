"use strict";

const { expect } = require("chai");
const { NATIVE_SURFACE_RUNTIME, isNativeSurface } = require("../../scripts/lib/native-surface");

function NativeSurfaceDetect() {
  const runtime =
    "0x363d3d373360601b36523d3d601436013d3d6110175af13d600060003e6024573d6000fd5b3d6000f3";

  expect(NATIVE_SURFACE_RUNTIME).to.equal(runtime);
  expect((runtime.length - 2) / 2).to.equal(41);
  expect(isNativeSurface(runtime)).to.equal(true);
  expect(isNativeSurface(runtime.toUpperCase())).to.equal(true);
  expect(isNativeSurface(new String(runtime))).to.equal(true);

  for (const code of [
    "0x",
    "",
    runtime + "00",
    "0x00" + runtime.slice(2),
    runtime.slice(0, -2),
    runtime.slice(2),
    " " + runtime,
    runtime + "\n",
    null,
    undefined,
    0,
  ]) {
    expect(isNativeSurface(code), String(code)).to.equal(false);
  }

  for (let offset = 2; offset < runtime.length; offset += 2) {
    const changedByte = runtime.slice(offset, offset + 2) === "00" ? "01" : "00";
    const changedRuntime = runtime.slice(0, offset) + changedByte + runtime.slice(offset + 2);
    expect(isNativeSurface(changedRuntime), changedRuntime).to.equal(false);
  }
}

it("NativeSurfaceDetect: matches only the 41-byte surface runtime", NativeSurfaceDetect);
