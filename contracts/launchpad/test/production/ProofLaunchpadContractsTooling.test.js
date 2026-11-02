/*
Proves: upgrade-preflight.upgradePreflight / production-health.inspectHealth / set-creation-fee.main -> Node/Hardhat subprocesses + JSON-RPC -> qualifyUpgrade / production-health.main / ProtocolConfig.creationFee.
Consumers: event-emitter-v2/{01-deploy-impl,02-queue-upgrade,02b-direct-upgrade,04-wire-registries}.qualifyUpgrade,
upgrade.qualifyUpgrade, upgrade-launchpad-optical.qualifyUpgrade, production-health.main,
and ProtocolConfig.creationFee.

Repairs: the two lib commands silently succeeded without running their producers;
the upgrade consumers executed on import and could not consume a fresh command result;
preflight accepted malformed migration bytes; set-creation-fee logged success without
asserting its postcondition. Every plan below pins real deployed code, compiler layouts
and a successful, rolled-back upgrade transaction. No transport or contract is doubled.
The anchor has no persistent state: commands reopen their inputs on every invocation.
*/
"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const crypto = require("node:crypto");
const { execFile } = require("node:child_process");
const { promisify } = require("node:util");
const execute = promisify(execFile);
const packageRoot = path.resolve(__dirname, "../..");
const consumerPaths = [
  "event-emitter-v2/01-deploy-impl.js",
  "event-emitter-v2/02-queue-upgrade.js",
  "event-emitter-v2/02b-direct-upgrade.js",
  "event-emitter-v2/04-wire-registries.js",
  "upgrade.js",
  "upgrade-launchpad-optical.js",
];

async function ProofLaunchpadContractsTooling() {
  const hre = require("hardhat");
  const { ethers, artifacts, network } = hre;
  const { JsonRpcServer } = require("hardhat/internal/hardhat-network/jsonrpc/server");
  const { atomicWrite } = require("../../scripts/lib/operations");
  const { decodePreflightResult } = require("../../scripts/lib/upgrade-preflight");
  const { IMPLEMENTATION_SLOT } = require("../../scripts/lib/production-health");
  const temporary = fs.mkdtempSync(path.join(os.tmpdir(), "launchpad-tooling-proof-"));
  const savedEnvironment = { ...process.env };
  const snapshot = await network.provider.send("evm_snapshot");
  const server = new JsonRpcServer({ hostname: "127.0.0.1", port: 0, provider: network.provider });
  try {
    const { port } = await server.listen();
    const rpc = `http://127.0.0.1:${port}`;
    const [admin, outsider] = await ethers.getSigners();
    for (const name of [
      "CONTRACT",
      "BEACON",
      "PROXY_ADDRESS",
      "IMPL_ADDRESS",
      "TIMELOCK_ADDRESS",
      "NETWORK_TYPE",
      "HARDHAT_NETWORK",
    ])
      delete process.env[name];
    process.env.RPC_URL = rpc;
    const manifestFile = path.join(temporary, "manifest.json");
    process.env.LAUNCHPAD_MANIFEST = manifestFile;
    const chainId = String((await ethers.provider.getNetwork()).chainId);
    const sha = (file) => crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex");
    const write = (name, value) => {
      const file = path.join(temporary, name);
      atomicWrite(file, value);
      return file;
    };
    const command = async (args, extra = {}) =>
      execute(process.execPath, args, {
        cwd: packageRoot,
        env: { ...process.env, ...extra },
        timeout: 60000,
        maxBuffer: 4 * 1024 * 1024,
      });
    const Proxy = await ethers.getContractFactory("UUPSProxy");
    async function deployProxy(name, initialize) {
      const Factory = await ethers.getContractFactory(name);
      const implementation = await Factory.deploy();
      await implementation.waitForDeployment();
      const proxy = await Proxy.deploy(
        await implementation.getAddress(),
        Factory.interface.encodeFunctionData("initialize", initialize)
      );
      await proxy.waitForDeployment();
      return { Factory, implementation, proxy, contract: Factory.attach(await proxy.getAddress()) };
    }
    const emitter = await deployProxy("EventEmitter", [admin.address]);
    // ProtocolConfig requires code at the quote-token address; this proof never uses token behavior.
    const config = await deployProxy("ProtocolConfig", [
      await emitter.implementation.getAddress(),
      ethers.ZeroAddress,
      admin.address,
    ]);
    const accumulator = await deployProxy("FeeAccumulator", [
      await config.proxy.getAddress(),
      admin.address,
      ethers.ZeroAddress,
      ethers.ZeroAddress,
      await config.proxy.getAddress(),
      admin.address,
    ]);
    const manifest = { _meta: { chainId }, _deployment: { records: {} } };
    for (const [name, deployment] of [
      ["EventEmitter", emitter],
      ["ProtocolConfig", config],
      ["FeeAccumulator", accumulator],
    ]) {
      const address = await deployment.proxy.getAddress();
      const implementation = await deployment.implementation.getAddress();
      manifest[`${name}_proxy`] = address;
      manifest[`${name}_impl`] = implementation;
      manifest._deployment.records[`${name}_proxy`] = {
        name: "UUPSProxy",
        address,
        args: [implementation],
        runtimeHash: ethers.keccak256(await ethers.provider.getCode(address)),
      };
      manifest._deployment.records[`${name}_impl`] = {
        name,
        address: implementation,
        runtimeHash: ethers.keccak256(await ethers.provider.getCode(implementation)),
      };
    }
    atomicWrite(manifestFile, manifest);
    const plans = {};
    for (const [name, deployment] of [
      ["EventEmitter", emitter],
      ["FeeAccumulator", accumulator],
    ]) {
      const replacement = await deployment.Factory.deploy();
      await replacement.waitForDeployment();
      const migrationCalldata =
        name === "EventEmitter"
          ? deployment.Factory.interface.encodeFunctionData("reinitializeV2", [admin.address])
          : "0x";
      const beforeUpgrade = await network.provider.send("evm_snapshot");
      const transaction = await deployment.contract.upgradeToAndCall(
        await replacement.getAddress(),
        migrationCalldata
      );
      const receipt = await transaction.wait();
      const word = await ethers.provider.getStorage(
        await deployment.proxy.getAddress(),
        IMPLEMENTATION_SLOT
      );
      assert.equal(ethers.getAddress("0x" + word.slice(-40)), await replacement.getAddress());
      if (name === "EventEmitter")
        assert.equal(
          await deployment.contract.hasRole(ethers.id("EVENT_EMITTER_ROLE"), admin.address),
          true
        );
      const simulationFile = write(`${name}-simulation.json`, {
        chainId,
        transactionHash: receipt.hash,
        status: receipt.status,
        implementationSlot: word,
        blockNumber: receipt.blockNumber,
      });
      assert.equal(receipt.status, 1);
      assert.equal(await network.provider.send("evm_revert", [beforeUpgrade]), true);
      const artifact = await artifacts.readArtifact(name);
      const info = await artifacts.getBuildInfo(`${artifact.sourceName}:${name}`);
      const baseline = write(
        `${name}-layout.json`,
        info.output.contracts[artifact.sourceName][name].storageLayout
      );
      const plan = {
        chainId,
        migrationCalldata,
        simulation: { file: simulationFile, sha256: sha(simulationFile) },
        contracts: [
          {
            contract: name,
            proxyKey: `${name}_proxy`,
            oldImplementation: await deployment.implementation.getAddress(),
            oldCodeHash: ethers.keccak256(
              await ethers.provider.getCode(await deployment.implementation.getAddress())
            ),
            baseline,
            baselineSha256: sha(baseline),
            newArtifactHash: ethers.keccak256(artifact.bytecode),
          },
        ],
      };
      plans[name] = { plan, file: write(`${name}-plan.json`, plan) };
    }

    for (const relative of consumerPaths) {
      const name = relative === "upgrade-launchpad-optical.js" ? "FeeAccumulator" : "EventEmitter";
      process.env.UPGRADE_PLAN = plans[name].file;
      if (relative === "upgrade.js") process.env.CONTRACT = name;
      else delete process.env.CONTRACT;
      const consumer = require(path.join(packageRoot, "scripts", relative));
      assert.equal(typeof consumer.main, "function");
      for (let attempt = 0; attempt < 2; attempt++) {
        const observed = await consumer.qualifyUpgrade();
        assert.deepEqual(observed, plans[name].plan, relative);
        assert.notEqual(observed, plans[name].plan);
      }
      // Rewriting the pinned input between fresh commands proves there is no cached approval.
      atomicWrite(plans[name].file, { ...plans[name].plan, chainId: "1" });
      await assert.rejects(consumer.qualifyUpgrade(), /Upgrade plan chain mismatch/);
      atomicWrite(plans[name].file, plans[name].plan);
      assert.deepEqual(await consumer.qualifyUpgrade(), plans[name].plan);
    }
    delete process.env.CONTRACT;
    process.env.UPGRADE_PLAN = plans.EventEmitter.file;
    const preflightCommand = path.join(packageRoot, "scripts/lib/upgrade-preflight.js");
    const produced = await command([preflightCommand, "EventEmitter"]);
    assert.deepEqual(
      decodePreflightResult(produced.stdout, ["EventEmitter"]),
      plans.EventEmitter.plan
    );
    assert.throws(
      () => decodePreflightResult(produced.stdout.slice(0, -3), ["EventEmitter"]),
      /Malformed upgrade preflight result/
    );
    const malformed = JSON.parse(produced.stdout);
    malformed.migrationCalldata = "0x0";
    assert.throws(
      () => decodePreflightResult(JSON.stringify(malformed), ["EventEmitter"]),
      /Malformed upgrade preflight result/
    );
    atomicWrite(plans.EventEmitter.file, malformed);
    await assert.rejects(
      command([preflightCommand, "EventEmitter"]),
      /Malformed upgrade preflight result/
    );
    atomicWrite(plans.EventEmitter.file, plans.EventEmitter.plan);
    await assert.rejects(command([preflightCommand, "UnknownContract"]), /contract set differs/);

    const policyFile = write("health-policy.json", {
      requiredContracts: ["ProtocolConfig_proxy"],
      expectedRoles: [
        {
          contract: "ProtocolConfig_proxy",
          role: ethers.ZeroHash,
          account: outsider.address,
          present: false,
        },
      ],
    });
    const healthCommand = path.join(packageRoot, "scripts/lib/production-health.js");
    let priorBlock = -1;
    for (let attempt = 0; attempt < 2; attempt++) {
      await network.provider.send("evm_mine");
      const result = JSON.parse((await command([healthCommand, manifestFile, policyFile])).stdout);
      assert.equal(result.status, "ok");
      assert.equal(result.chainId, chainId);
      assert.ok(result.blockNumber > priorBlock);
      priorBlock = result.blockNumber;
      assert.equal(result.checks.find((check) => check.id === "snapshot:canonical").status, "ok");
      assert.equal(
        result.checks.find((check) => check.id === "proxy:ProtocolConfig_proxy").detail.actual,
        await config.implementation.getAddress()
      );
    }
    await (await config.contract.grantRole(ethers.ZeroHash, outsider.address)).wait();
    await assert.rejects(command([healthCommand, manifestFile, policyFile]), (error) => {
      assert.equal(error.code, 1);
      const result = JSON.parse(error.stdout);
      assert.equal(result.status, "critical");
      assert.equal(
        result.checks.find((check) => check.id.startsWith("role:")).detail.present,
        true
      );
      return true;
    });
    await (await config.contract.revokeRole(ethers.ZeroHash, outsider.address)).wait();
    write("health-policy.json", { requiredContract: ["ProtocolConfig_proxy"] });
    await assert.rejects(command([healthCommand, manifestFile, policyFile]), (error) => {
      assert.equal(error.code, 1);
      assert.equal(JSON.parse(error.stderr).status, "critical");
      return true;
    });

    // Use the package's real Hardhat CLI/config with only an ephemeral local network added.
    const configFile = path.join(temporary, "hardhat.config.cjs");
    fs.writeFileSync(
      configFile,
      `const config = require(${JSON.stringify(path.join(packageRoot, "hardhat.config.js"))});\nmodule.exports = { ...config, paths: { root: ${JSON.stringify(packageRoot)} }, networks: { ...config.networks, proof: { url: ${JSON.stringify(rpc)} } } };\n`
    );
    const hardhat = require.resolve("hardhat/internal/cli/cli");
    const feeArgs = [
      hardhat,
      "--config",
      configFile,
      "run",
      "--no-compile",
      "scripts/set-creation-fee.js",
      "--network",
      "proof",
    ];
    await (await config.contract.setCreationFee(5_000_000n)).wait();
    // Default account is now the wrong signer: the actual contract enforces the refusal.
    await (await config.contract.grantRole(ethers.ZeroHash, outsider.address)).wait();
    await (await config.contract.revokeRole(ethers.ZeroHash, admin.address)).wait();
    await assert.rejects(command(feeArgs), (error) => {
      assert.equal(error.code, 1);
      assert.match(error.stderr, /MissingRole|revert/);
      return true;
    });
    assert.equal(await config.contract.creationFee(), 5_000_000n);
    await (
      await config.contract.connect(outsider).grantRole(ethers.ZeroHash, admin.address)
    ).wait();
    const changed = await command(feeArgs);
    assert.match(changed.stdout, /Creation fee updated/);
    assert.equal(await config.contract.creationFee(), 1_000_000n);
    const nonce = await ethers.provider.getTransactionCount(admin.address);
    const repeated = await command(feeArgs);
    assert.match(repeated.stdout, /Fee is already \$1/);
    assert.equal(await config.contract.creationFee(), 1_000_000n);
    assert.equal(await ethers.provider.getTransactionCount(admin.address), nonce);
    await (await config.contract.setCreationFee(3_000_000n)).wait();
    assert.match((await command(feeArgs)).stdout, /Creation fee updated/);
    assert.equal(await config.contract.creationFee(), 1_000_000n);
  } finally {
    await server.close();
    await network.provider.send("evm_revert", [snapshot]);
    for (const key of Object.keys(process.env))
      if (!(key in savedEnvironment)) delete process.env[key];
    Object.assign(process.env, savedEnvironment);
    fs.rmSync(temporary, { recursive: true, force: true });
  }
}

if (process.env.LAUNCHPAD_PROOF_NODE_TEST === "1") {
  require("node:test").test(ProofLaunchpadContractsTooling);
} else {
  // Hardhat's declared gate compiles first; node:test owns and awaits the actual proof.
  it("ProofLaunchpadContractsTooling", async function () {
    this.timeout(180000);
    const result = await execute(process.execPath, ["--test", __filename], {
      cwd: packageRoot,
      env: { ...process.env, LAUNCHPAD_PROOF_NODE_TEST: "1", HARDHAT_NETWORK: "hardhat" },
      timeout: 170000,
      maxBuffer: 4 * 1024 * 1024,
    });
    process.stdout.write(result.stdout);
    process.stderr.write(result.stderr);
  });
}
