import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { createRequire, isBuiltin } from "node:module";
import { dirname, join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const testsDirectory = dirname(fileURLToPath(import.meta.url));
const workspaceDirectory = dirname(testsDirectory);
const requireFromTests = createRequire(import.meta.url);
const manifest = JSON.parse(
  readFileSync(join(workspaceDirectory, "package.json"), "utf8"),
) as {
  dependencies?: Record<string, string>;
  devDependencies: Record<string, string>;
  scripts: Record<string, string>;
};

function sourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return sourceFiles(path);
    return entry.isFile() && /\.(?:[cm]?[jt]s|[jt]sx)$/.test(entry.name)
      ? [path]
      : [];
  });
}

function importedModules(path: string): string[] {
  const source = ts.createSourceFile(
    path,
    readFileSync(path, "utf8"),
    ts.ScriptTarget.Latest,
    true,
  );
  const imports = new Set<string>();

  function visit(node: ts.Node) {
    let specifier: ts.Node | undefined;
    if (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) {
      specifier = node.moduleSpecifier;
    } else if (
      ts.isImportEqualsDeclaration(node) &&
      ts.isExternalModuleReference(node.moduleReference)
    ) {
      specifier = node.moduleReference.expression;
    } else if (
      ts.isImportTypeNode(node) &&
      ts.isLiteralTypeNode(node.argument)
    ) {
      specifier = node.argument.literal;
    } else if (
      ts.isCallExpression(node) &&
      (node.expression.kind === ts.SyntaxKind.ImportKeyword ||
        (ts.isIdentifier(node.expression) &&
          node.expression.text === "require"))
    ) {
      specifier = node.arguments[0];
    }
    if (specifier && ts.isStringLiteralLike(specifier)) {
      imports.add(specifier.text);
    }
    ts.forEachChild(node, visit);
  }

  visit(source);
  return [...imports];
}

function rootTestsDeclareAndResolveImports() {
  const dependencies = {
    ...manifest.dependencies,
    ...manifest.devDependencies,
  };
  const files = sourceFiles(testsDirectory);
  assert.ok(files.length > 0, "The tests directory contains source files");

  for (const file of files) {
    for (const specifier of importedModules(file)) {
      if (specifier.startsWith(".") || isBuiltin(specifier)) continue;
      const parts = specifier.split("/");
      const packageName = specifier.startsWith("@")
        ? parts.slice(0, 2).join("/")
        : parts[0];
      const context = `${relative(workspaceDirectory, file)} imports ${specifier}`;
      assert.ok(
        Object.hasOwn(dependencies, packageName),
        `${context}: ${packageName} is declared at the workspace root`,
      );
      assert.doesNotThrow(
        () => requireFromTests.resolve(packageName),
        `${context}: ${packageName} resolves from the tests directory`,
      );
      assert.doesNotThrow(
        () => requireFromTests.resolve(specifier),
        `${context}: the imported path resolves from the tests directory`,
      );
    }
  }
}

function rootTestDependenciesMatchWebApplication() {
  const webManifest = JSON.parse(
    readFileSync(join(workspaceDirectory, "apps/web/package.json"), "utf8"),
  ) as { dependencies: Record<string, string> };

  for (const packageName of [
    "@tanstack/react-query",
    "react",
    "react-dom",
    "next",
  ]) {
    assert.ok(Object.hasOwn(manifest.devDependencies, packageName));
    assert.equal(
      manifest.devDependencies[packageName],
      webManifest.dependencies[packageName],
      `${packageName} uses the web application's version`,
    );
  }
}

function typecheckBuildsWorkspaceDependencies() {
  assert.deepEqual(manifest.scripts.typecheck.split(/\s*&&\s*/), [
    "pnpm --filter @sidioralabs/chainflow-data-gateway build",
    "tsc --noEmit -p apps/web/tsconfig.json",
  ]);
}

test(rootTestsDeclareAndResolveImports);
test(rootTestDependenciesMatchWebApplication);
test(typecheckBuildsWorkspaceDependencies);
