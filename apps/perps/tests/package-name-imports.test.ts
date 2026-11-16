import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const testsDirectory = fileURLToPath(new URL(".", import.meta.url));
const workspaceDirectory = join(testsDirectory, "..");
const sourceRoots = [
  join(workspaceDirectory, "apps"),
  join(workspaceDirectory, "packages"),
];
const sourceExtensions = /\.(?:[cm]?[jt]s|[jt]sx)$/;

function isBuildOutput(name: string): boolean {
  return (
    name === "node_modules" ||
    name === "dist" ||
    name === "build" ||
    name === "out" ||
    name === "coverage" ||
    name === ".turbo" ||
    name === "storybook-static" ||
    name.startsWith(".next")
  );
}

function sourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    if (entry.isDirectory() && !isBuildOutput(entry.name)) {
      return sourceFiles(join(directory, entry.name));
    }
    if (!entry.isFile() || !sourceExtensions.test(entry.name)) return [];
    const path = join(directory, entry.name);
    return /^\/\/ File generated\b/.test(readFileSync(path, "utf8"))
      ? []
      : [path];
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

  function visit(node: ts.Node): void {
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

function importsThroughModuleDirectoriesAreRejected(): void {
  const moduleDirectoryImport = /(?:^|[\\/])node_modules(?:[\\/]|$)/;
  const files = sourceRoots.flatMap(sourceFiles);
  assert.ok(files.length > 0, "workspace source files are present");

  const violations: string[] = [];
  for (const file of files) {
    for (const specifier of importedModules(file)) {
      if (moduleDirectoryImport.test(specifier)) {
        violations.push(
          `${relative(workspaceDirectory, file)} imports ${specifier}`,
        );
      }
    }
  }

  assert.deepEqual(violations, []);
}

function webReporterDependencyUsesBackendRange(): void {
  const webManifest = JSON.parse(
    readFileSync(join(workspaceDirectory, "apps/web/package.json"), "utf8"),
  ) as { dependencies: Record<string, string> };
  const backendManifest = JSON.parse(
    readFileSync(
      join(workspaceDirectory, "packages/paxeer-backend/package.json"),
      "utf8",
    ),
  ) as { dependencies: Record<string, string> };

  assert.equal(
    webManifest.dependencies["@sentry/nextjs"],
    backendManifest.dependencies["@sentry/nextjs"],
  );
}

test(importsThroughModuleDirectoriesAreRejected);
test(webReporterDependencyUsesBackendRange);
