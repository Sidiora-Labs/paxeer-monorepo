const { creationChainRoots } = require('./creation-chain-layout.cjs');
const roots = creationChainRoots(
  process.env.L27_SOURCE_ROOT,
  process.env.CONTRACT_SOURCE_ROOT,
);
module.exports = {
  networks: { lane: roots.network },
  paths: { cache: roots.cache, artifacts: roots.contracts + '/artifacts' },
};
