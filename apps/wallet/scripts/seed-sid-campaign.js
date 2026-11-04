#!/usr/bin/env node
// Run with the same PUSH_DATA_DIR and filesystem identity as the push service.
const path = require('node:path');

const SID_CAMPAIGN = Object.freeze({
  id: 'sid-airdrop-2026',
  title: '$SID Airdrop Starts Tomorrow!',
  body: 'The largest $SID airdrop on Paxeer Network begins tomorrow. Hold $SID in your wallet to qualify.',
  url: '/',
  tag: 'sid-airdrop',
});

async function seedSidCampaign(store, parseCampaign, input = SID_CAMPAIGN) {
  const campaign = parseCampaign(input);
  try {
    await store.addCampaign(campaign);
    return 'created';
  } catch (error) {
    if (!(error instanceof Error) || error.message !== 'Campaign already exists') throw error;
    const existing = await store.getCampaign(campaign.id);
    const fields = [
      'id',
      'title',
      'body',
      'url',
      'icon',
      'image',
      'tag',
      'scheduledAt',
      'targetTags',
      'targetAddresses',
    ];
    if (
      !existing ||
      fields.some((key) => JSON.stringify(existing[key]) !== JSON.stringify(campaign[key]))
    ) {
      throw new Error('Campaign id belongs to a different announcement');
    }
    return 'existing';
  }
}

async function main() {
  const { createServer } = await import('vite');
  const loader = await createServer({
    configFile: false,
    root: path.resolve(__dirname, '..'),
    publicDir: false,
    logLevel: 'error',
    appType: 'custom',
    server: { middlewareMode: true, hmr: false, ws: false },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  try {
    const { pushStore, parseCampaign } = await loader.ssrLoadModule('/src/server/push-store.ts');
    const result = await seedSidCampaign(pushStore, parseCampaign);
    console.log(
      result === 'created' ? 'Seeded SID campaign.' : 'SID campaign already exists. Skipping.'
    );
  } finally {
    await loader.close();
  }
}

module.exports = { SID_CAMPAIGN, seedSidCampaign };

if (require.main === module) {
  main().catch((error) => {
    console.error('Campaign seeding failed:', error.message);
    process.exitCode = 1;
  });
}
