import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join, resolve, sep } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const webRoot = fileURLToPath(new URL("../", import.meta.url));
const sourceRoot = join(webRoot, "src");
const configPath = join(webRoot, "tsconfig.json");
const configFile = ts.readConfigFile(configPath, ts.sys.readFile);
assert.equal(configFile.error, undefined);
const config = ts.parseJsonConfigFileContent(configFile.config, ts.sys, webRoot);
assert.deepEqual(config.errors, []);

function moduleImports(source: ts.SourceFile): readonly string[] {
  const imports: string[] = [];
  function visit(node: ts.Node): void {
    if (
      (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
      node.moduleSpecifier !== undefined &&
      ts.isStringLiteralLike(node.moduleSpecifier)
    ) {
      imports.push(node.moduleSpecifier.text);
    } else if (
      ts.isImportEqualsDeclaration(node) &&
      ts.isExternalModuleReference(node.moduleReference) &&
      node.moduleReference.expression !== undefined &&
      ts.isStringLiteralLike(node.moduleReference.expression)
    ) {
      imports.push(node.moduleReference.expression.text);
    } else if (
      ts.isCallExpression(node) &&
      (node.expression.kind === ts.SyntaxKind.ImportKeyword ||
        (ts.isIdentifier(node.expression) && node.expression.text === "require"))
    ) {
      const specifier = node.arguments[0];
      if (specifier !== undefined && ts.isStringLiteralLike(specifier)) {
        imports.push(specifier.text);
      }
    } else if (
      ts.isImportTypeNode(node) &&
      ts.isLiteralTypeNode(node.argument) &&
      ts.isStringLiteralLike(node.argument.literal)
    ) {
      imports.push(node.argument.literal.text);
    }
    ts.forEachChild(node, visit);
  }
  visit(source);
  return imports;
}

function no_client_module_imports_custody_handoff() {
  const handoff = join(sourceRoot, "journeys", "custody", "handoff.ts");
  for (const directory of ["app", "settings"]) {
    const files = ts.sys.readDirectory(join(sourceRoot, directory), [
      ".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs",
    ]);
    assert.ok(files.length > 0, `${directory} must contain client modules`);
    for (const file of files) {
      const source = ts.createSourceFile(
        file, readFileSync(file, "utf8"), ts.ScriptTarget.Latest, true,
      );
      for (const specifier of moduleImports(source)) {
        const resolved = ts.resolveModuleName(specifier, file, config.options, ts.sys).resolvedModule;
        if (specifier.startsWith(".")) {
          assert.ok(
            resolved !== undefined || ts.sys.fileExists(resolve(dirname(file), specifier)),
            `${file}: cannot resolve ${specifier}`,
          );
        }
        assert.notEqual(
          resolve(dirname(file), specifier).replace(/\.(?:[cm]?[jt]sx?)$/u, ""),
          handoff.slice(0, -3),
          `${file} imports custody handoff through ${specifier}`,
        );
        if (resolved !== undefined) {
          assert.notEqual(
            resolve(resolved.resolvedFileName), handoff,
            `${file} resolves ${specifier} to custody handoff`,
          );
        }
      }
    }
  }
}

test(no_client_module_imports_custody_handoff);

function exchange_and_launchpad_clients_load_without_custody() {
  const custodyRoot = join(sourceRoot, "journeys", "custody") + sep;
  const providerPath = join(sourceRoot, "api", "provider.ts");
  for (const client of ["exchange", "launchpad"]) {
    const entry = join(sourceRoot, "app", client, `${client}-client.tsx`);
    const program = ts.createProgram([entry], config.options);
    assert.ok(program.getSourceFile(entry) !== undefined, `${client} entry must load`);
    const provider = program.getSourceFile(providerPath);
    assert.ok(provider !== undefined, `${client} must load the shared wallet provider`);
    const windowWalletProvider = provider.statements.find((statement) =>
      ts.isFunctionDeclaration(statement) && statement.name?.text === "windowWalletProvider",
    );
    assert.ok(
      windowWalletProvider !== undefined,
      "the shared provider must declare windowWalletProvider",
    );
    assert.deepEqual(
      program.getSourceFiles()
        .filter((source) => resolve(source.fileName).startsWith(custodyRoot))
        .map((source) => source.fileName),
      [],
      `${client} import graph must not load any custody module`,
    );
  }
}

test(exchange_and_launchpad_clients_load_without_custody);
