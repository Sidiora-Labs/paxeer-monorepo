const { overrides } = require('./package.json').pnpm;

function updateConfig(config) {
  return { ...config, overrides: { ...config.overrides, ...overrides } };
}

module.exports = { hooks: { updateConfig } };
