const { readFileSync, writeFileSync } = require('node:fs');

const port = process.env.PORT ?? '3001';
if (!/^[1-9][0-9]{0,4}$/.test(port) || Number(port) < 1024 || Number(port) > 65535) {
  throw new Error('PORT must be an integer between 1024 and 65535');
}
const appPort = port === '3000' ? '3001' : '3000';
const template = readFileSync(process.argv[2], 'utf8');
const listener = /\blisten 3001;/g;
const upstream = /\bserver 127\.0\.0\.1:3000;/g;
if ([...template.matchAll(listener)].length !== 1 || [...template.matchAll(upstream)].length !== 1) {
  throw new Error('Wallet proxy template must declare one listener and one application upstream');
}
const config = template
  .replace(listener, `listen ${port};`)
  .replace(upstream, `server 127.0.0.1:${appPort};`);
writeFileSync(process.argv[3], config, { mode: 0o600 });
process.stdout.write(appPort);
