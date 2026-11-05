import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
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

function explorer_specs_open_no_verify_page() {
  for (const path of ["./explorer.spec.ts", "./browser/explorer.spec.ts"]) {
    const source = readFileSync(new URL(path, import.meta.url), "utf8");
    assert.match(source, /["']\/explorer["']/u, `${path} must keep the explorer overview`);
    assert.doesNotMatch(source, /\/explorer\/verify\b/u, `${path} still names the verify page`);
    assert.doesNotMatch(source, /explorer\.verify\.title/u, `${path} still requires verifier copy`);
  }
}

function web_test_scripts_name_no_verifier_suite() {
  const webRoot = new URL("../", import.meta.url);
  const { scripts } = JSON.parse(readFileSync(new URL("package.json", webRoot), "utf8")) as {
    scripts: Record<string, string>;
  };
  for (const name of ["test", "test:component", "test:explorer"]) {
    assert.ok(Object.hasOwn(scripts, name), `missing ${name} script`);
    assert.ok(
      scripts[name]?.split(/\s+/u).includes("e2e/explorer-suites.test.ts"),
      `${name} must run the explorer suite regression tests`,
    );
  }
  for (const [name, script] of Object.entries(scripts)) {
    assert.ok(!script.includes("e2e/explorer-verifier.test.ts"), `${name} still names the verifier suite`);
    if (name !== "test" && !name.startsWith("test:")) continue;
    for (const argument of script.split(/\s+/u)) {
      if (/\.(?:[cm]?[jt]sx?|json)$/u.test(argument)) {
        assert.ok(existsSync(new URL(argument, webRoot)), `${name} names a missing file: ${argument}`);
      }
    }
  }
  assert.ok(!existsSync(new URL("e2e/explorer-verifier.test.ts", webRoot)), "the verifier suite must be deleted");
}

test(explorer_specs_open_no_verify_page);
test(web_test_scripts_name_no_verifier_suite);
