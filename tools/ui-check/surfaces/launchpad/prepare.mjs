import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync, chmodSync, readFileSync, writeFileSync } from 'node:fs';
import { homedir, tmpdir } from 'node:os';
import { join } from 'node:path';

export const certificateDirectory = join(tmpdir(), 'launchpad-ui-certificates');
export const certificate = join(certificateDirectory, 'sidiora.test.pem');
export const privateKey = join(certificateDirectory, 'sidiora.test-key.pem');

execFileSync('ip', ['address', 'replace', '203.0.113.1/32', 'dev', 'lo']);
const hosts = readFileSync('/etc/hosts', 'utf8').split('\n').filter(line => !line.trim().split(/\s+/).slice(1).includes('sidiora.test'));
writeFileSync('/etc/hosts', hosts.join('\n').trimEnd() + '\n203.0.113.1 sidiora.test\n');

mkdirSync(certificateDirectory, { recursive: true, mode: 0o700 });
if (!existsSync(certificate) || !existsSync(privateKey)) {
  execFileSync('openssl', [
    'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '2',
    '-subj', '/CN=sidiora.test', '-addext', 'subjectAltName=DNS:sidiora.test',
    '-addext', 'basicConstraints=critical,CA:TRUE',
    '-keyout', privateKey, '-out', certificate,
  ], { stdio: 'pipe' });
  chmodSync(privateKey, 0o600);
}
const database = join(homedir(), '.pki', 'nssdb');
mkdirSync(database, { recursive: true, mode: 0o700 });
if (!existsSync(join(database, 'cert9.db'))) {
  execFileSync('certutil', ['-N', '--empty-password', '-d', `sql:${database}`]);
}
execFileSync('certutil', [
  '-A', '-d', `sql:${database}`, '-n', 'Launchpad sidiora.test surface',
  '-t', 'C,,', '-i', certificate,
]);
console.log(`Local TLS certificate: ${certificate}`);
