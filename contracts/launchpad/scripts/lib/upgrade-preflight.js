"use strict";
const fs = require("fs");
const crypto = require("crypto");
const { manifestPath, validateManifest } = require("./operations");
const { compareLayouts, readRequiredBaseline } = require("./storage-layout");
const { assertNotNativeSurface } = require("./native-surface");
// A reviewed, chain-specific plan is mandatory even for deployment-only upgrade stages.
async function upgradePreflight(requiredContracts, options = {}) {
  const hre = require("hardhat");
  const provider = options.provider || hre.ethers.provider;
  if (!process.env.UPGRADE_PLAN)
    throw new Error(
      "UPGRADE_PLAN required: pinned old code, artifacts, layouts, migration and simulation evidence"
    );
  const plan = decodePreflightResult(
    fs.readFileSync(process.env.UPGRADE_PLAN, "utf8"),
    requiredContracts
  );
  const addresses = JSON.parse(
    fs.readFileSync(
      options.manifestFile || process.env.LAUNCHPAD_MANIFEST || manifestPath(),
      "utf8"
    )
  );
  await validateManifest(addresses, provider);
  if (String(plan.chainId) !== String((await provider.getNetwork()).chainId))
    throw new Error("Upgrade plan chain mismatch");
  const evidence = fs.readFileSync(plan.simulation.file);
  if (crypto.createHash("sha256").update(evidence).digest("hex") !== plan.simulation.sha256)
    throw new Error("Simulation evidence hash mismatch");
  for (const item of plan.contracts) {
    if (process.env.BEACON === "true" && item.contract !== "SidioraPool")
      throw new Error("Beacon upgrade restricted to SidioraPool");
    if (process.env.CONTRACT && item.contract !== process.env.CONTRACT)
      throw new Error("Contract differs from reviewed plan");
    if (item.proxyKey !== (item.beacon ? "PoolBeacon" : `${item.contract}_proxy`))
      throw new Error("Plan proxy key does not match contract identity");
    if (item.beacon && item.contract !== "SidioraPool")
      throw new Error("Wrong beacon implementation");
    if (process.env.PROXY_ADDRESS || process.env.IMPL_ADDRESS || process.env.TIMELOCK_ADDRESS)
      throw new Error(
        "Address overrides bypass reviewed identity; update the pinned manifest/plan instead"
      );
    const proxy = addresses[item.proxyKey];
    if (!proxy || !item.oldImplementation || !item.oldCodeHash)
      throw new Error("Incomplete implementation identity");
    await assertNotNativeSurface(provider, proxy, item.proxyKey);
    let actual;
    if (item.beacon)
      actual = await (
        await hre.ethers.getContractAt("PoolBeacon", proxy, provider)
      ).implementation();
    else {
      const slot = "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc";
      actual = hre.ethers.getAddress("0x" + (await provider.getStorage(proxy, slot)).slice(-40));
    }
    if (actual.toLowerCase() !== item.oldImplementation.toLowerCase())
      throw new Error("Stale implementation identity");
    if (hre.ethers.keccak256(await provider.getCode(actual)) !== item.oldCodeHash)
      throw new Error("Old implementation code hash mismatch");
    if (item.beacon) {
      if (!item.tokenInventory)
        throw new Error(
          "Beacon upgrade requires complete reviewed immutable-token compatibility inventory"
        );
      const raw = fs.readFileSync(item.tokenInventory.file);
      if (crypto.createHash("sha256").update(raw).digest("hex") !== item.tokenInventory.sha256)
        throw new Error("Token inventory hash mismatch");
      const inventory = JSON.parse(raw);
      const registry = await hre.ethers.getContractAt(
        "PoolRegistry",
        addresses.PoolRegistry_proxy,
        provider
      );
      const count = Number(await registry.getPoolCount());
      if (!Array.isArray(inventory) || inventory.length !== count)
        throw new Error("Incomplete pool-token inventory");
      const seen = new Set();
      const block = await provider.getBlockNumber();
      for (let offset = 0; offset < count; offset += 100) {
        const pools = await registry.getAllPools(offset, Math.min(100, count - offset));
        for (const pool of pools) {
          const record = inventory.find((x) => x.pool.toLowerCase() === pool.toLowerCase());
          if (!record || seen.has(pool.toLowerCase()))
            throw new Error("Missing or duplicate pool inventory");
          seen.add(pool.toLowerCase());
          const livePool = await hre.ethers.getContractAt("SidioraPool", pool, provider);
          const tokenAddress = await livePool.tokenAddress();
          if (
            tokenAddress.toLowerCase() !== record.token.toLowerCase() ||
            hre.ethers.keccak256(await provider.getCode(tokenAddress)) !== record.runtimeHash
          )
            throw new Error("Immutable token identity mismatch");
          const token = await hre.ethers.getContractAt("SidioraERC20", tokenAddress, provider);
          if ((await token.pool()).toLowerCase() !== pool.toLowerCase())
            throw new Error("Legacy token lacks bound transfer hooks");
          await token.getPastBalance(hre.ethers.ZeroAddress, block - 1);
          await token.getPastTotalSupply(block - 1);
          if (!record.transferHookSimulation) throw new Error("Missing transfer-hook fork proof");
        }
      }
    }
    const old = readRequiredBaseline(item.baseline);
    if (
      crypto.createHash("sha256").update(fs.readFileSync(item.baseline)).digest("hex") !==
      item.baselineSha256
    )
      throw new Error("Pinned baseline hash mismatch");
    const artifact = await hre.artifacts.readArtifact(item.contract);
    if (hre.ethers.keccak256(artifact.bytecode) !== item.newArtifactHash)
      throw new Error("New artifact hash mismatch");
    const info = await hre.artifacts.getBuildInfo(
      `${artifact.sourceName}:${artifact.contractName}`
    );
    const current = info.output.contracts[artifact.sourceName][artifact.contractName].storageLayout;
    const errors = compareLayouts(old, current);
    if (errors.length) throw new Error(`Incompatible storage: ${errors.join("; ")}`);
  }
  return plan;
}
function decodePreflightResult(output, requiredContracts) {
  let plan;
  try {
    plan = JSON.parse(output);
  } catch {
    throw new Error("Malformed upgrade preflight result: invalid JSON");
  }
  if (
    !Array.isArray(requiredContracts) ||
    !requiredContracts.length ||
    requiredContracts.some((name) => typeof name !== "string" || !name)
  )
    throw new Error("Upgrade script requires exact target contracts");
  if (
    !plan ||
    typeof plan !== "object" ||
    Array.isArray(plan) ||
    !/^[1-9][0-9]*$/.test(String(plan.chainId)) ||
    typeof plan.migrationCalldata !== "string" ||
    !/^0x(?:[0-9a-fA-F]{2})*$/.test(plan.migrationCalldata) ||
    !plan.simulation ||
    typeof plan.simulation.file !== "string" ||
    !plan.simulation.file ||
    !/^[0-9a-f]{64}$/.test(plan.simulation.sha256) ||
    !Array.isArray(plan.contracts) ||
    plan.contracts.some((item) => !item || typeof item.contract !== "string")
  )
    throw new Error("Malformed upgrade preflight result: incomplete qualification plan");
  if (
    JSON.stringify([...requiredContracts].sort()) !==
    JSON.stringify(plan.contracts.map((item) => item.contract).sort())
  )
    throw new Error("Upgrade plan contract set differs from script targets");
  return plan;
}

// Keep qualification in a fresh process so every stage re-reads the reviewed inputs.
async function runUpgradePreflight(requiredContracts) {
  const hre = require("hardhat");
  if (
    !Array.isArray(requiredContracts) ||
    !requiredContracts.length ||
    requiredContracts.some((name) => typeof name !== "string" || !name)
  )
    throw new Error("Upgrade script requires exact target contracts");
  const rpc = process.env.RPC_URL || hre.network.config.url;
  if (!rpc) throw new Error("RPC_URL or a connected HTTP network required for upgrade preflight");
  const { execFile } = require("node:child_process");
  const { promisify } = require("node:util");
  const { stdout } = await promisify(execFile)(
    process.execPath,
    [__filename, ...requiredContracts],
    {
      cwd: hre.config.paths.root,
      env: { ...process.env, RPC_URL: rpc, HARDHAT_NETWORK: hre.network.name },
      timeout: 60000,
      maxBuffer: 4 * 1024 * 1024,
    }
  );
  const plan = decodePreflightResult(stdout, requiredContracts);
  if (String(plan.chainId) !== String((await hre.ethers.provider.getNetwork()).chainId))
    throw new Error("Upgrade preflight result differs from connected chain");
  return plan;
}

async function main(requiredContracts = process.argv.slice(2)) {
  if (!process.env.RPC_URL) throw new Error("RPC_URL required for upgrade preflight command");
  const provider = new (require("ethers").JsonRpcProvider)(process.env.RPC_URL);
  try {
    const plan = await upgradePreflight(requiredContracts, { provider });
    console.log(JSON.stringify(plan));
    return plan;
  } finally {
    provider.destroy();
  }
}

module.exports = { upgradePreflight, runUpgradePreflight, decodePreflightResult, main };
if (require.main === module)
  main().catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
