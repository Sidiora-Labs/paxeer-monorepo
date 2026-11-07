// Generate executable fixture assets from the repository's actual contracts.
// npm install --prefix /tmp/r509-solc --no-save --package-lock=false solc@0.8.27
// node daemon/committed-fixture/generate_v6_contracts.cjs /tmp/r509-solc/node_modules/solc
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const solc = require(path.resolve(process.argv[2]));
const delegate = 'contracts/src/BatchCallAndSponsor.sol';
const token = 'tools/benchmark/loadtest/contracts/evm/src/ERC20Token.sol';
const settings = {
  optimizer: { enabled: true, runs: 200 },
  evmVersion: 'cancun',
  outputSelection: { '*': { '*': ['abi', 'evm.bytecode.object', 'evm.deployedBytecode.object', 'evm.deployedBytecode.immutableReferences'], '': ['ast'] } },
};
const sources = Object.fromEntries([delegate, token].map(source => [source, { content: fs.readFileSync(source, 'utf8') }]));
const output = JSON.parse(solc.compile(JSON.stringify({ language: 'Solidity', sources, settings }), {
  import: source => {
    const prefix = '@openzeppelin/contracts/';
    const file = source.startsWith(prefix)
      ? 'tools/benchmark/loadtest/contracts/evm/lib/openzeppelin-contracts/contracts/' + source.slice(prefix.length)
      : source;
    return { contents: fs.readFileSync(file, 'utf8') };
  },
}));
for (const error of output.errors ?? []) {
  if (error.severity === 'error') throw new Error(error.formattedMessage);
}
const immutableNames = {};
const visit = node => {
  if (!node || typeof node !== 'object') return;
  if (node.nodeType === 'VariableDeclaration' && node.mutability === 'immutable') immutableNames[node.id] = node.name;
  for (const value of Object.values(node)) {
    if (Array.isArray(value)) value.forEach(visit);
    else if (value && typeof value === 'object') visit(value);
  }
};
Object.values(output.sources).forEach(source => visit(source.ast));
const asset = (source, name) => {
  const c = output.contracts[source][name];
  return {
    source,
    sourceSha256: crypto.createHash('sha256').update(sources[source].content).digest('hex'),
    abi: c.abi,
    creationCode: '0x' + c.evm.bytecode.object,
    runtimeCode: '0x' + c.evm.deployedBytecode.object,
    immutableReferences: Object.fromEntries(Object.entries(c.evm.deployedBytecode.immutableReferences ?? {}).map(([id, spans]) => [immutableNames[id], spans])),
  };
};
fs.writeFileSync('daemon/committed-fixture/v6_contracts.json', JSON.stringify({
  compiler: solc.version(), settings,
  delegate: asset(delegate, 'BatchCallAndSponsor'),
  token: asset(token, 'ERC20Token'),
}, null, 2) + '\n');
