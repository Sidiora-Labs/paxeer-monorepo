const { overrides } = require('./package.json');

function updateConfig(config) {
  return { ...config, overrides: { ...config.overrides, ...overrides } };
}

module.exports = { hooks: { updateConfig } };
