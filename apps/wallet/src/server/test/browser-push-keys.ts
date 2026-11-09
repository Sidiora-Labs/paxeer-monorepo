import { createECDH, randomBytes } from 'node:crypto';

export function browserPushKeys(): { p256dh: string; auth: string } {
  const browser = createECDH('prime256v1');
  return {
    p256dh: browser.generateKeys().toString('base64url'),
    auth: randomBytes(16).toString('base64url'),
  };
}
