'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const reviewedFile = path.resolve(__dirname, '../../../../../../sdk/launchpad/shared/config/deployment/current.json');

function createDeploymentGenerator(base) {
  function deriveDeployment(contractRoot) {
    const data = base.deriveDeployment(contractRoot);
    const manifest = JSON.parse(fs.readFileSync(path.join(contractRoot, 'deployments/paxeer-x-network-addresses.json'), 'utf8'));
    const reviewed = JSON.parse(fs.readFileSync(reviewedFile, 'utf8'));
    assert.equal(reviewed.chainId, data.chainId, 'Reviewed deployment chain mismatch');

    const treasury = manifest.Treasury_proxy;
    assert.match(treasury, /^0x[0-9a-fA-F]{40}$/, 'Missing manifest role Treasury_proxy');
    assert.notEqual(treasury.toLowerCase(), '0x' + '0'.repeat(40));
    const record = manifest._deployment.records.Treasury_proxy;
    assert.equal(record?.address?.toLowerCase(), treasury.toLowerCase(), 'Treasury_proxy:record address mismatch');
    assert.equal(record?.status, 'confirmed', 'Treasury_proxy:deployment record not confirmed');
    const treasuryIdentity = reviewed.roles.Treasury;
    assert.equal(treasuryIdentity?.address?.toLowerCase(), treasury.toLowerCase(), 'Treasury_proxy:current deployment role address mismatch');
    assert.equal(treasuryIdentity.kind, 'surface', 'Treasury:unsupported role kind');
    assert.equal(treasuryIdentity.runtimeHash, data.roleIdentity.router.runtimeHash, 'Treasury:surface runtime hash mismatch');

    const usdlIdentity = reviewed.roles.USDL;
    assert.equal(usdlIdentity?.address?.toLowerCase(), data.usdl.toLowerCase(), 'USDL:current deployment role address mismatch');
    assert.equal(usdlIdentity.kind, 'direct', 'USDL:unsupported role kind');
    assert.match(usdlIdentity.runtimeHash, /^0x[0-9a-fA-F]{64}$/, 'USDL:missing runtime hash');
    assert.notEqual(usdlIdentity.runtimeHash, '0x' + '0'.repeat(64), 'USDL:zero runtime hash');

    const precompileIdentity = reviewed.roles.LaunchpadPrecompile;
    assert.equal(precompileIdentity?.kind, 'precompile', 'LaunchpadPrecompile:unsupported role kind');
    assert.equal(precompileIdentity.address, '0x0000000000000000000000000000000000001017', 'LaunchpadPrecompile:unexpected address');
    assert.equal(reviewed.sourceManifestSha256, data.manifestSha256, 'Reviewed deployment manifest mismatch');

    return {
      ...data,
      treasury,
      launchpadPrecompile: precompileIdentity.address,
      roleIdentity: {
        ...data.roleIdentity,
        treasury: { kind: treasuryIdentity.kind, runtimeHash: treasuryIdentity.runtimeHash },
        usdl: { kind: usdlIdentity.kind, runtimeHash: usdlIdentity.runtimeHash },
        launchpadPrecompile: { kind: precompileIdentity.kind },
      },
    };
  }

  function renderHelper(data) {
    return base.renderHelper(data) + '\n' +
      "export const CORE_TRADE_EXECUTION_ROLES = Object.freeze(['router', 'quoter', 'poolRegistry', 'eventEmitter'] as const);\n" +
      "export const PRIVATE_FIXTURE_WAGMI_STORAGE_KEY = 'kindle-private-qualification-v1';\n";
  }

  function checkClient(clientRoot, data, helper) {
    return base.checkClient(clientRoot, data, helper);
  }

  return { deriveDeployment, renderHelper, checkClient };
}

module.exports = { createDeploymentGenerator };
