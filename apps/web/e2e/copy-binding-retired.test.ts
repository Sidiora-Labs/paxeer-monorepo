import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { registerHooks } from "node:module";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js")
          ? new URL(candidate.href.slice(0, -3) + suffix)
          : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module",
        shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: {
            target: ts.ScriptTarget.ES2022,
            module: ts.ModuleKind.ESNext,
            jsx: ts.JsxEmit.ReactJSX,
          },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const copyEntry = (await import("../copy/runtime.ts")).copyEntry;
const { copyEntries } = await import("../copy/catalog.ts");

function binding_copy_keys_are_gone() {
  const retired = [
    "none",
    "binding",
    "bound",
    "rebinding",
    "summary",
    "unbound.body",
    "binding.body",
    "no_authority",
    "current",
    "address",
    "qr",
    "linked_at",
    "verification",
    "receipt_verified",
    "active.body",
    "change",
    "retry",
    "cancel",
    "add_money",
    "pending_address",
    "handoff.in_progress",
    "handoff.in_progress.body",
    "handoff.cancelled.body",
    "handoff.rejected.body",
    "handoff.unavailable.body",
    "handoff.failed.body",
    "rebinding.body",
    "security_notification.body",
  ];
  for (const suffix of retired) {
    const key = `settings.wallet.${suffix}`;
    assert.throws(() => copyEntry(key), { message: `Unknown copy key: ${key}` });
  }
  assert.deepEqual(
    copyEntries.filter(({ key }) => key.startsWith("settings.wallet.")).map(({ key }) => key),
    ["settings.wallet.title"],
  );
  assert.deepEqual(copyEntry("settings.wallet.title"), {
    key: "settings.wallet.title",
    message: "Payout wallet",
  });
  execFileSync(process.execPath, ["copy/generate-runtime.ts", "--check"], {
    cwd: fileURLToPath(new URL("../", import.meta.url)),
    stdio: "pipe",
  });
}

function every_settings_wallet_key_in_source_is_catalogued() {
  const sourceRoot = fileURLToPath(new URL("../src/", import.meta.url));
  let references = 0;
  for (const filename of readdirSync(sourceRoot, { recursive: true, encoding: "utf8" })) {
    if (!/\.[cm]?[jt]sx?$/u.test(filename)) continue;
    const source = ts.createSourceFile(
      filename,
      readFileSync(join(sourceRoot, filename), "utf8"),
      ts.ScriptTarget.Latest,
      true,
    );
    function visit(node: ts.Node): void {
      if (ts.isStringLiteralLike(node) && node.text.startsWith("settings.wallet.")) {
        assert.equal(copyEntry(node.text).key, node.text, `${filename}: ${node.text}`);
        references += 1;
      }
      ts.forEachChild(node, visit);
    }
    visit(source);
  }
  assert.ok(references > 0, "Source must still reference the retained wallet copy");
}

test(binding_copy_keys_are_gone);
test(every_settings_wallet_key_in_source_is_catalogued);
