import { createRequire } from "node:module";
import { readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath, pathToFileURL } from "node:url";

const projects = [
  ["contracts", "hardhat.config.js", "0.8.27"],
  ["tests/integration/dapp_tests", "hardhat.config.js", "0.8.28"],
  ["tests/integration/rpc_tests", "hardhat.config.ts", "0.8.28"],
];

export async function compilerRequirements(root = process.cwd()) {
  return Promise.all(
    projects.map(async ([directory, config, version]) => {
      const source = await readFile(path.join(root, directory, config), "utf8");
      const match = source.match(
        /\bsolidity\s*:\s*\{\s*version\s*:\s*["'](\d+\.\d+\.\d+)["']/
      );
      if (!match)
        throw new Error(
          `${directory}/${config}: expected one Solidity version`
        );
      if (match[1] !== version) {
        throw new Error(
          `${directory}/${config}: expected prepared Solidity version ${version}, got ${match[1]}`
        );
      }
      return { directory, version };
    })
  );
}

async function checkCompilers() {
  for (const { directory, version } of await compilerRequirements()) {
    const require = createRequire(
      pathToFileURL(path.resolve(directory, "package.json"))
    );
    const {
      CompilerDownloader,
      CompilerPlatform,
    } = require("hardhat/internal/solidity/compiler/downloader");
    const { getCompilersDir } = require("hardhat/internal/util/global-dir");
    const compilersDir = await getCompilersDir();
    const platforms = [
      CompilerDownloader.getCompilerPlatform(),
      CompilerPlatform.WASM,
    ];
    for (const platform of new Set(platforms)) {
      const downloader = CompilerDownloader.getConcurrencySafeDownloader(
        platform,
        compilersDir
      );
      if (!(await downloader.isCompilerDownloaded(version))) {
        console.error(
          `${directory}: Hardhat solc ${version} for ${platform} is not installed`
        );
        process.exitCode = 1;
      }
    }
  }
}

if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  await checkCompilers();
}
