import assert from "node:assert/strict";
import { existsSync, readFileSync, statSync } from "node:fs";
import { registerHooks } from "node:module";
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

async function retired_journey_modules_do_not_resolve() {
  for (const journey of ["deposit", "withdraw", "exit"]) {
    for (const filename of ["index.ts", "model.ts", "screen.tsx"]) {
      const source = new URL(`../src/journeys/${journey}/${filename}`, import.meta.url);
      await assert.rejects(
        () => import(source.href),
        { code: "ERR_MODULE_NOT_FOUND", url: source.href },
        `${journey}/${filename} must no longer resolve`,
      );
    }
  }
}

function web_test_list_names_no_custody_suite() {
  const manifest = JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf8"));
  for (const name of ["test", "test:component"]) {
    const script = manifest.scripts[name];
    assert.equal(typeof script, "string");
    const args = script.trim().split(/\s+/u);
    assert.deepEqual(args.slice(0, 2), ["node", "--test"]);
    const files = args.slice(2);
    assert.ok(files.length > 0, `${name} must name test files`);
    for (const retired of ["e2e/custody.spec.ts", "e2e/custody-evidence.test.ts"]) {
      assert.ok(!script.includes(retired), `${name} must not name ${retired}`);
    }
    for (const filename of files) {
      const source = new URL(`../${filename}`, import.meta.url);
      assert.ok(statSync(source).isFile(), `${name} names a missing test file: ${filename}`);
    }
  }
}

test(retired_journey_modules_do_not_resolve);
test(web_test_list_names_no_custody_suite);
