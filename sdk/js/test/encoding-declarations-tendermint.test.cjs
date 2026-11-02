const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const ts = require("typescript");

function tendermint_public_encoding_declarations_match_retained_surface() {
  const packageRoot = path.resolve(__dirname, "..");
  const consumerRoot = fs.mkdtempSync(
    path.join(os.tmpdir(), "pax-sdk-encoding-"),
  );
  try {
    const packageScope = path.join(consumerRoot, "node_modules", "@pax-js");
    fs.mkdirSync(packageScope, { recursive: true });
    fs.symlinkSync(packageRoot, path.join(packageScope, "cosmos"), "junction");
    const fileName = path.join(consumerRoot, "encoding-probe.ts");
    fs.writeFileSync(
      fileName,
      `
import { Encoder } from "@pax-js/cosmos/encoding";
import * as abci from "@pax-js/cosmos/types/tendermint/abci";

const response = Encoder.tendermint.abci.ResponseCheckTx.create({
  code: 0, data: new Uint8Array([1]), gas_wanted: 100,
  codespace: "", sender: "pax1sender", priority: 1,
});
const encoded: Uint8Array = Encoder.tendermint.abci.ResponseCheckTx.encode(response).finish();
const decoded = Encoder.tendermint.abci.ResponseCheckTx.decode(encoded);
const gasWanted: number = decoded.gas_wanted;

type AssertFalse<T extends false> = T;
type NoRetiredEnum = AssertFalse<"CheckTxType" extends keyof typeof abci ? true : false>;
type NoRetiredFromJSON = AssertFalse<"checkTxTypeFromJSON" extends keyof typeof Encoder.tendermint.abci ? true : false>;
type NoRetiredToJSON = AssertFalse<"checkTxTypeToJSON" extends keyof typeof Encoder.tendermint.abci ? true : false>;
`,
    );
    const options = {
      strict: true,
      noEmit: true,
      skipLibCheck: false,
      esModuleInterop: true,
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.CommonJS,
      moduleResolution: ts.ModuleResolutionKind.Node10,
      types: [],
    };
    const resolved = ts.resolveModuleName(
      "@pax-js/cosmos/encoding",
      fileName,
      options,
      ts.sys,
    ).resolvedModule;
    assert.ok(resolved);
    assert.equal(
      fs.realpathSync(resolved.resolvedFileName),
      path.join(packageRoot, "dist", "types", "encoding", "index.d.ts"),
    );
    const program = ts.createProgram([fileName], options);
    const diagnostics = ts.getPreEmitDiagnostics(program);
    assert.deepEqual(
      diagnostics.map((diagnostic) => ({
        file: diagnostic.file?.fileName,
        code: diagnostic.code,
        message: ts.flattenDiagnosticMessageText(diagnostic.messageText, "\n"),
      })),
      [],
    );
  } finally {
    fs.rmSync(consumerRoot, { recursive: true, force: true });
  }
}

test(tendermint_public_encoding_declarations_match_retained_surface);
