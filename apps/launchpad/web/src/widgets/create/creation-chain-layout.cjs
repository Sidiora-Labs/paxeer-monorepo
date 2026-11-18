// eslint-disable-next-line @typescript-eslint/no-require-imports -- The chain configuration loads this module through CommonJS.
const path = require('node:path');
// eslint-disable-next-line @typescript-eslint/no-require-imports -- The chain configuration loads this module through CommonJS.
const layout = require('./creation-chain-layout.json');

function creationChainRoots(sourceRoot = '', contractRoot = '') {
  const source = path.resolve(
    sourceRoot || path.resolve(__dirname, layout.repositoryFromWidget),
  );
  return {
    source,
    contracts: path.resolve(
      contractRoot || path.join(source, layout.contracts),
    ),
    web: path.join(source, layout.web),
    cache: path.join(__dirname, '.private-cache'),
    network: { ...layout.network },
  };
}

module.exports = { creationChainRoots };
