const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

function hardhatSolcMatchesFoundryPaxeer() {
  const originalCwd = process.cwd();
  try {
    process.chdir(path.join(__dirname, ".."));
    const hre = require("hardhat");
    const foundryConfig = fs.readFileSync(
      path.join(__dirname, "../../foundry.paxeer.toml"),
      "utf8",
    );
    const solcVersion = foundryConfig.match(/^solc_version\s*=\s*"([^"]+)"\s*$/m);
    assert.ok(solcVersion, "foundry.paxeer.toml must declare solc_version");
    assert.equal(hre.config.solidity.compilers[0].version, solcVersion[1]);
  } finally {
    process.chdir(originalCwd);
  }
}

test(hardhatSolcMatchesFoundryPaxeer);
