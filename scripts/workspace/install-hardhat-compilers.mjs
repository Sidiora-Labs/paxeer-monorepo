import { createRequire } from "node:module";
import { randomUUID } from "node:crypto";
import { chmod, mkdir, readFile, rename, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath, pathToFileURL } from "node:url";
import { compilerRequirements } from "./check-hardhat-compilers.mjs";

const repositoryUrl = "https://binaries.soliditylang.org";

async function download(url) {
  const response = await fetch(url, { signal: AbortSignal.timeout(120_000) });
  if (!response.ok) throw new Error(`${url}: HTTP ${response.status}`);
  return Buffer.from(await response.arrayBuffer());
}

function compilerDigest(bytes, require) {
  const { keccak256 } = require("ethereum-cryptography/keccak");
  return `0x${Buffer.from(keccak256(bytes)).toString("hex")}`;
}

export async function installCompiler({ version, platform, compilersDir, baseUrl, require }) {
  const platformDir = path.join(compilersDir, platform);
  const listPath = path.join(platformDir, "list.json");
  let listBytes;
  try {
    listBytes = await readFile(listPath);
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
    listBytes = await download(`${baseUrl}/${platform}/list.json`);
    await mkdir(platformDir, { recursive: true });
    await writeFile(listPath, listBytes);
  }

  const list = JSON.parse(listBytes.toString("utf8"));
  const build = list.builds?.find((candidate) => candidate.version === version);
  if (!build || !/^[\w.+-]+$/.test(build.path) || !/^0x[0-9a-fA-F]{64}$/.test(build.keccak256)) {
    throw new Error(`Hardhat solc ${version} for ${platform}: missing published build or digest`);
  }
  const compilerPath = path.join(platformDir, build.path);
  try {
    const present = await readFile(compilerPath);
    if (compilerDigest(present, require).toLowerCase() === build.keccak256.toLowerCase()) return;
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }

  const compiler = await download(`${baseUrl}/${platform}/${build.path}`);
  if (compilerDigest(compiler, require).toLowerCase() !== build.keccak256.toLowerCase()) {
    throw new Error(`Hardhat solc ${version} for ${platform} (${build.path}): digest mismatch`);
  }
  const temporaryPath = `${compilerPath}.${randomUUID()}.tmp`;
  try {
    await writeFile(temporaryPath, compiler);
    if (platform !== "wasm") await chmod(temporaryPath, 0o755);
    await rename(temporaryPath, compilerPath);
  } finally {
    await rm(temporaryPath, { force: true });
  }
}

async function installHardhatCompilers() {
  const installed = new Set();
  for (const { directory, version } of await compilerRequirements()) {
    const require = createRequire(pathToFileURL(path.resolve(directory, "package.json")));
    const { CompilerDownloader, CompilerPlatform } = require("hardhat/internal/solidity/compiler/downloader");
    const { getCompilersDir } = require("hardhat/internal/util/global-dir");
    const compilersDir = await getCompilersDir();
    for (const platform of new Set([CompilerDownloader.getCompilerPlatform(), CompilerPlatform.WASM])) {
      const key = `${compilersDir}:${platform}:${version}`;
      if (installed.has(key)) continue;
      await installCompiler({ version, platform, compilersDir, baseUrl: repositoryUrl, require });
      installed.add(key);
    }
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  await installHardhatCompilers();
}
