const { expect } = require("chai");
const fs = require("fs");
const os = require("os");
const path = require("path");
const crypto = require("crypto");
const { ethers, network } = require("hardhat");
const {
  atomicWrite,
  validateManifest,
  networkName,
  manifestPath,
} = require("../../scripts/lib/operations");
const { upgradePreflight } = require("../../scripts/lib/upgrade-preflight");
const {
  NATIVE_SURFACE_RUNTIME,
  assertNotNativeSurface,
} = require("../../scripts/lib/native-surface");

async function NativeSurfaceUpgradeRefused() {
  const snapshot = await network.provider.send("evm_snapshot");
  const file = manifestPath();
  const previousManifest = fs.existsSync(file) ? fs.readFileSync(file) : null;
  const previousPlan = process.env.UPGRADE_PLAN;
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "native-upgrade-"));
  try {
    const proxy = "0x1111111111111111111111111111111111111111";
    const proxyKey = "Router_proxy";
    const implementation = await (await ethers.getContractFactory("Router")).deploy();
    await implementation.waitForDeployment();
    const oldImplementation = await implementation.getAddress();
    const oldCode = await ethers.provider.getCode(oldImplementation);
    await assertNotNativeSurface(ethers.provider, oldImplementation, proxyKey);
    await network.provider.send("hardhat_setCode", [proxy, NATIVE_SURFACE_RUNTIME]);
    const chainId = String((await ethers.provider.getNetwork()).chainId);
    const slot = "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc";
    const implementationSlot = await ethers.provider.getStorage(proxy, slot);
    expect(implementationSlot).to.equal(ethers.ZeroHash);
    const evidence = JSON.stringify({
      chainId,
      proxy,
      code: await ethers.provider.getCode(proxy),
      implementationSlot,
    });
    const evidenceFile = path.join(dir, "evidence.json");
    fs.writeFileSync(evidenceFile, evidence);
    const planFile = path.join(dir, "plan.json");
    fs.writeFileSync(
      planFile,
      JSON.stringify({
        chainId,
        migrationCalldata: "0x",
        simulation: {
          file: evidenceFile,
          sha256: crypto.createHash("sha256").update(evidence).digest("hex"),
        },
        contracts: [
          {
            contract: "Router",
            proxyKey,
            oldImplementation,
            oldCodeHash: ethers.keccak256(oldCode),
          },
        ],
      })
    );
    atomicWrite(file, { _meta: { network: chainId }, [proxyKey]: proxy });
    process.env.UPGRADE_PLAN = planFile;
    await expect(upgradePreflight(["Router"])).to.be.rejectedWith(
      Error,
      `Native surface ${proxyKey}: upgrade through governance MsgUpdateParams`
    );
    await network.provider.send("hardhat_setCode", [proxy, oldCode]);
    await expect(upgradePreflight(["Router"])).to.be.rejectedWith(
      Error,
      "Stale implementation identity"
    );
  } finally {
    if (previousPlan === undefined) delete process.env.UPGRADE_PLAN;
    else process.env.UPGRADE_PLAN = previousPlan;
    if (previousManifest === null) fs.rmSync(file, { force: true });
    else fs.writeFileSync(file, previousManifest);
    fs.rmSync(dir, { recursive: true });
    await network.provider.send("evm_revert", [snapshot]);
  }
}

describe("OPS operator safety", function () {
  afterEach(() => {
    delete process.env.NETWORK_TYPE;
    delete process.env.UPGRADE_PLAN;
  });
  it("rejects caller-supplied wrong network labels", () => {
    process.env.NETWORK_TYPE = "paxeer-network";
    expect(() => networkName()).to.throw("disagrees");
  });
  it("rejects absent or mismatched manifest chains", async () => {
    const provider = { getNetwork: async () => ({ chainId: 125n }) };
    for (const manifest of [{}, { _meta: { network: "31337" } }]) {
      let error;
      try {
        await validateManifest(manifest, provider);
      } catch (e) {
        error = e;
      }
      expect(error.message).to.include("chain ID");
    }
  });
  it("rejects manifest addresses without code", async () => {
    const provider = { getNetwork: async () => ({ chainId: 125n }), getCode: async () => "0x" };
    let error;
    try {
      await validateManifest(
        { _meta: { network: "125" }, Router_proxy: "0x1111111111111111111111111111111111111111" },
        provider
      );
    } catch (e) {
      error = e;
    }
    expect(error.message).to.include("No contract code");
  });
  it("writes recoverable complete atomic checkpoints", () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "ops-checkpoint-"));
    const file = path.join(dir, "manifest.json");
    try {
      atomicWrite(file, { stage: 1 });
      atomicWrite(file, { stage: 2 });
      expect(JSON.parse(fs.readFileSync(file))).to.deep.equal({ stage: 2 });
      expect(fs.readdirSync(dir)).to.deep.equal(["manifest.json"]);
    } finally {
      fs.rmSync(dir, { recursive: true });
    }
  });
  it("rejects upgrades before transactions when no reviewed plan exists", async () => {
    let error;
    try {
      await upgradePreflight(["Router"]);
    } catch (e) {
      error = e;
    }
    expect(error.message).to.include("UPGRADE_PLAN required");
  });
  it("NativeSurfaceUpgradeRefused: preflight refuses native surfaces", NativeSurfaceUpgradeRefused);
});
