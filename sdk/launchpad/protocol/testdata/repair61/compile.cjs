const fs = require('fs'), path = require('path');
const [root, output] = process.argv.slice(2);
const solc = require(path.join(output, 'node_modules/solc'));
const base = path.join(root, 'contracts/launchpad');
const entries = {
  SidioraNFT: ['contracts/core/SidioraNFT.sol', 'SidioraNFT'],
  FeesRouter: ['contracts/periphery/FeesRouter.sol', 'FeesRouter'],
  ERC1967Proxy: ['contracts/meta-ag/proxy/ERC1967Proxy.sol', 'ERC1967Proxy'],
};
const sources = {};
function load(file) {
  if (sources[file]) return;
  const content = fs.readFileSync(path.join(base, file), 'utf8');
  sources[file] = {content};
  for (const match of content.matchAll(/import\s+(?:[^;]*?from\s+)?["']([^"']+)["']\s*;/g)) {
    load(path.posix.normalize(path.posix.join(path.posix.dirname(file), match[1])));
  }
}
for (const [file] of Object.values(entries)) load(file);
const settings = {optimizer: {enabled: true, runs: 200}, viaIR: true, outputSelection: {'*': {'*': ['abi', 'evm.bytecode.object']}}};
const compiled = JSON.parse(solc.compile(JSON.stringify({language: 'Solidity', sources, settings})));
for (const error of compiled.errors || []) if (error.severity === 'error') throw Error(error.formattedMessage);
for (const [name, [file, contract]] of Object.entries(entries)) {
  const artifact = compiled.contracts[file][contract];
  fs.writeFileSync(path.join(output, name + '.json'), JSON.stringify({abi: artifact.abi, bytecode: '0x' + artifact.evm.bytecode.object}));
}
