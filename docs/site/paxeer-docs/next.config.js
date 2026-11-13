const { createHash } = require('node:crypto')
const { readFileSync } = require('node:fs')
const path = require('node:path')

const layerOrder = '@layer properties, supplied, theme, base, components, utilities;\n'

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  transpilePackages: ['@centra/docs-assistant'],
  images: { unoptimized: true },
  trailingSlash: true,
  webpack(config, { webpack }) {
    if (config.cache && typeof config.cache === 'object') {
      const source = path.join(__dirname, 'node_modules/@centra/docs-assistant/src')
      const fingerprint = createHash('sha256')
        .update(readFileSync(path.join(source, 'DocsAssistant.tsx')))
        .update(readFileSync(path.join(source, 'style.css')))
        .update(readFileSync(__filename))
        .update(readFileSync(path.join(__dirname, 'postcss.config.mjs')))
        .digest('hex')
      config.cache.version = `${config.cache.version || ''}:docs-assistant:${fingerprint}`
    }
    config.plugins.push({
      apply(compiler) {
        compiler.hooks.thisCompilation.tap('PaxeerSdkLayerOrder', compilation => {
          compilation.hooks.processAssets.tap({
            name: 'PaxeerSdkLayerOrder',
            stage: webpack.Compilation.PROCESS_ASSETS_STAGE_SUMMARIZE,
          }, assets => {
            for (const [name, source] of Object.entries(assets)) {
              if (name.endsWith('.css')) {
                compilation.updateAsset(name, new webpack.sources.ConcatSource(layerOrder, source))
              }
            }
          })
        })
      },
    })
    return config
  },
}

module.exports = nextConfig
