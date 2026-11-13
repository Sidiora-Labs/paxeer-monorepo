'use strict';

const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const generator = require('../../../../../../tools/launchpad/production/interfaces/contracts/client-deployment.cjs');

function deriveDeployment(contractRoot) {
  const data = generator.deriveDeployment(contractRoot);
  const manifest = JSON.parse(fs.readFileSync(path.resolve(__dirname,
    '../../../../shared/config/deployment/current.json'), 'utf8'));
  const roles = { ...generator.ROLE_KEYS, treasury: 'Treasury_proxy', usdl: 'USDL',
    launchpadPrecompile: 'LaunchpadPrecompile' };
  for (const [key, role] of Object.entries(roles)) {
    const identity = manifest.roles[role.replace(/_proxy$/, '')];
    const hasHash = identity.stagedRuntimeHash !== undefined;
    const hasUpgrade = identity.stagedUpgrade !== undefined;
    assert.equal(hasHash, hasUpgrade, `${role}:incomplete staged identity`);
    if (hasHash) {
      assert.equal(identity.kind, 'surface', `${role}:staged identity requires surface`);
      assert.match(identity.stagedRuntimeHash, /^0x[0-9a-f]{64}$/, `${role}:invalid staged hash`);
      assert.notEqual(identity.stagedRuntimeHash, '0x' + '0'.repeat(64));
      assert.notEqual(identity.stagedRuntimeHash, identity.runtimeHash);
      assert.match(identity.stagedUpgrade, /^[a-zA-Z0-9][a-zA-Z0-9._-]{0,95}$/, `${role}:invalid staged upgrade`);
      data.roleIdentity[key].stagedRuntimeHash = identity.stagedRuntimeHash;
      data.roleIdentity[key].stagedUpgrade = identity.stagedUpgrade;
    }
  }
  return data;
}

const { renderHelper, checkClient } = generator;
if (require.main === module) {
  const contractRoot = process.env.CONTRACT_SOURCE_ROOT;
  assert.ok(contractRoot, 'CONTRACT_SOURCE_ROOT required');
  const data = deriveDeployment(contractRoot);
  const helper = renderHelper(data);
  if (process.argv[2] === 'generate') {
    assert.ok(process.env.CLIENT_DEPLOYMENT_HELPER_OUTPUT, 'CLIENT_DEPLOYMENT_HELPER_OUTPUT required');
    fs.writeFileSync(process.env.CLIENT_DEPLOYMENT_HELPER_OUTPUT, helper);
  } else if (process.argv[2] === 'check') {
    assert.ok(process.env.CLIENT_SOURCE_ROOT, 'CLIENT_SOURCE_ROOT required');
    checkClient(process.env.CLIENT_SOURCE_ROOT, data, helper);
  } else {
    throw new Error('Use generate or check');
  }
}

module.exports = { deriveDeployment, renderHelper, checkClient };
