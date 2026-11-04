const assert = require("node:assert/strict");
const { readFileSync } = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const ts = require("typescript");

function readme_examples_compile_against_public_package() {
	const readme = readFileSync(path.join(__dirname, "../README.md"), "utf8");
	const examples = [...readme.matchAll(/^```typescript\n([\s\S]*?)^```/gm)];
	assert.ok(examples.length > 0, "README must contain TypeScript examples");
	for (const [index, example] of examples.entries()) {
		const fileName = path.join(__dirname, `readme-example-${index}.ts`);
		const options = {
			strict: true,
			noEmit: true,
			target: ts.ScriptTarget.ES2022,
			module: ts.ModuleKind.ESNext,
			moduleResolution: ts.ModuleResolutionKind.Bundler,
		};
		const host = ts.createCompilerHost(options);
		const getSourceFile = host.getSourceFile.bind(host);
		host.getSourceFile = (file, languageVersion, onError, shouldCreateNewSourceFile) =>
			file === fileName
				? ts.createSourceFile(file, example[1], languageVersion, true)
				: getSourceFile(file, languageVersion, onError, shouldCreateNewSourceFile);
		const program = ts.createProgram([fileName], options, host);
		const diagnostics = ts.getPreEmitDiagnostics(program);
		assert.equal(
			diagnostics.length,
			0,
			`README TypeScript block ${index + 1}:\n${ts.formatDiagnosticsWithColorAndContext(
				diagnostics,
				{
					getCanonicalFileName: (file) => file,
					getCurrentDirectory: () => __dirname,
					getNewLine: () => "\n",
				},
			)}`,
		);
	}
}

test(readme_examples_compile_against_public_package);
