'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {deriveDeployment,renderHelper,checkClient,ROLE_KEYS}=require('./client-deployment.cjs');
const clientRoot=path.resolve(__dirname,'../../../../../apps/launchpad/web/src');
const helper=require(path.join(clientRoot,'core/network/deployment.ts'));
const defaultRoot=path.resolve(__dirname,'../../../../../contracts/launchpad');
const data=deriveDeployment(process.env.CONTRACT_SOURCE_ROOT||defaultRoot);
function deriveDeploymentDefaultsToRepositoryContracts(){const old=process.env.CONTRACT_SOURCE_ROOT;try{delete process.env.CONTRACT_SOURCE_ROOT;const deployment=deriveDeployment(defaultRoot);assert.equal(deployment.chainId,125);assert.equal(deployment.router,'0x78FB06f525cD964388f2305A489035263E31C6e4');}finally{if(old===undefined)delete process.env.CONTRACT_SOURCE_ROOT;else process.env.CONTRACT_SOURCE_ROOT=old;}}
test(deriveDeploymentDefaultsToRepositoryContracts);
test('helper bytes and all identity fields derive from reviewed manifest',()=>{assert.equal(fs.readFileSync(path.join(clientRoot,'core/network/deployment.ts'),'utf8'),renderHelper(data));assert.deepEqual(helper.CORE_DEPLOYMENT,data);assert.equal(data.chainId,125);});
test('accepted router alone can receive approval, with case-insensitive identity',()=>{assert.equal(helper.isActiveApprovalSpender(data.router),true);assert.equal(helper.isActiveApprovalSpender('0x'+data.router.slice(2).toUpperCase()),true);helper.assertActiveApprovalSpender(data.router);});
test('stale router, optional spender, other role and malformed spender reject',()=>{for(const value of ['0xCC7298801112682e10ee14b8a520309caD80336d','0x16bC4aB280c67977639591615d2a52273cDf5f6E',data.feesRouter,'0x'+'0'.repeat(40),'not an address','',undefined]){assert.equal(helper.isActiveApprovalSpender(value),false);assert.throws(()=>helper.assertActiveApprovalSpender(value),/no verified active deployment/);}});
test('optional capabilities fail closed without fabricated addresses',()=>{assert.deepEqual(helper.OPTIONAL_CONTRACT_CAPABILITIES,{metaAg:false,pecorOrders:false});for(const capability of ['metaAg','pecorOrders'])assert.throws(()=>helper.assertOptionalContractCapability(capability),/no verified active deployment/);});
test('environment override cannot change frozen address or optional policy',()=>{const old=process.env.ROUTER_ADDRESS;try{process.env.ROUTER_ADDRESS='0xCC7298801112682e10ee14b8a520309caD80336d';assert.equal(helper.CORE_DEPLOYMENT.router,data.router);assert.equal(helper.isActiveApprovalSpender(process.env.ROUTER_ADDRESS),false);}finally{if(old===undefined)delete process.env.ROUTER_ADDRESS;else process.env.ROUTER_ADDRESS=old;}});
test('deployment and capability objects cannot be mutated',()=>{assert.throws(()=>{helper.CORE_DEPLOYMENT.router=data.feesRouter},TypeError);assert.throws(()=>{helper.OPTIONAL_CONTRACT_CAPABILITIES.metaAg=true},TypeError);});
test('checker rejects drift from its manifest-derived helper',()=>{const drift={...data,router:'0xCC7298801112682e10ee14b8a520309caD80336d'};assert.throws(()=>checkClient(clientRoot,drift,renderHelper(drift)),/helper drift/);});

function renderedHelperCarriesSurfaceRoleIdentity() {
  const current = JSON.parse(fs.readFileSync(path.resolve(__dirname, '../../../../../sdk/launchpad/shared/config/deployment/current.json'), 'utf8'));
  const expected = {};
  for (const [key, role] of Object.entries(ROLE_KEYS)) {
    const identity = current.roles[role.replace(/_proxy$/, '')];
    assert.equal(data[key].toLowerCase(), identity.address.toLowerCase());
    expected[key] = { kind: identity.kind, runtimeHash: identity.runtimeHash };
    if (identity.kind === 'erc1967' || identity.kind === 'beacon') {
      expected[key].implementationAddress = identity.implementationAddress;
      expected[key].implementationHash = identity.implementationHash;
    }
  }
  assert.deepEqual(data.roleIdentity, expected);
  assert.deepEqual(helper.CORE_DEPLOYMENT.roleIdentity, expected);
  for (const key of ['router', 'quoter', 'feesRouter', 'protocolConfig', 'factory', 'nft', 'feeAccumulator', 'poolRegistry']) {
    assert.deepEqual(helper.CORE_DEPLOYMENT.roleIdentity[key], {
      kind: 'surface',
      runtimeHash: '0x6302ff80f4168c2af23afbf3117c613edd66d288d120eee825f9ecadd43a392f',
    });
  }
  assert.equal(helper.CORE_DEPLOYMENT.roleIdentity.opticalRegistry.kind, 'erc1967');
  assert.equal(helper.CORE_DEPLOYMENT.roleIdentity.eventEmitter.kind, 'erc1967');
  assert.equal(helper.CORE_DEPLOYMENT.roleIdentity.poolBeacon.kind, 'beacon');
  const rendered = renderHelper(data);
  assert.equal(fs.readFileSync(path.join(clientRoot, 'core/network/deployment.ts'), 'utf8'), rendered);
  assert.equal(checkClient(clientRoot, data, rendered).checked, true);
}
test(renderedHelperCarriesSurfaceRoleIdentity);

function deriveDeploymentRejectsRoleAddressDrift() {
  const contractRoot = process.env.CONTRACT_SOURCE_ROOT || defaultRoot;
  const manifest = fs.readFileSync(path.join(contractRoot, 'deployments/paxeer-x-network-addresses.json'), 'utf8');
  const temporaryRoot = fs.mkdtempSync(path.join(require('node:os').tmpdir(), 'client-deployment-role-drift-'));
  try {
    fs.mkdirSync(path.join(temporaryRoot, 'deployments'));
    const manifestFile = path.join(temporaryRoot, 'deployments/paxeer-x-network-addresses.json');
    fs.writeFileSync(manifestFile, manifest);
    assert.deepEqual(deriveDeployment(temporaryRoot), data);
    for (const [key, role] of Object.entries(ROLE_KEYS)) {
      const drifted = JSON.parse(manifest);
      const address = key === 'router' ? data.quoter : data.router;
      drifted[role] = address;
      drifted._deployment.records[role].address = address;
      fs.writeFileSync(manifestFile, JSON.stringify(drifted));
      assert.throws(() => deriveDeployment(temporaryRoot), {
        name: 'AssertionError',
        message: new RegExp(`${role}:current deployment role address mismatch`),
      });
    }
    const caseVariant = JSON.parse(manifest);
    for (const role of Object.values(ROLE_KEYS)) {
      caseVariant[role] = '0x' + caseVariant[role].slice(2).toUpperCase();
      caseVariant._deployment.records[role].address = caseVariant[role];
    }
    fs.writeFileSync(manifestFile, JSON.stringify(caseVariant));
    assert.deepEqual(deriveDeployment(temporaryRoot).roleIdentity, data.roleIdentity);
  } finally {
    fs.rmSync(temporaryRoot, { recursive: true, force: true });
  }
}
test(deriveDeploymentRejectsRoleAddressDrift);
