import fs from 'node:fs';
import path from 'node:path';

import assert from 'node:assert/strict';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';

const repository = fileURLToPath(new URL('../../../../', import.meta.url));
const pages = path.join(repository, 'apps/explorer/pages');
const surface = JSON.parse(fs.readFileSync(path.join(repository, 'tools/ui-check/surfaces/explorer.json'), 'utf8'));

function pageRoutes(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    if (entry.name === 'api' || entry.name.startsWith('_')) {
      return [];
    }
    const filename = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      return pageRoutes(filename);
    }
    if (!entry.name.endsWith('.tsx')) {
      return [];
    }
    const relative = path.relative(pages, filename).replace(/\.tsx$/, '').replace(/(?:^|\/)index$/, '');
    return [ '/' + relative ];
  });
}

function explorerEveryPageHasADeviceView() {
  const routes = pageRoutes(pages).sort();
  const names = surface.views.map((view) => view.name);
  assert.equal(new Set(names).size, names.length);
  assert.deepEqual(names.sort(), routes);
  for (const view of surface.views) {
    const pattern = new RegExp('^/explorer' + view.name.replace(/[.*+?^$()|\\]/g, '\\$&').replace(/\[\w+\]/g, '[^/]+') + '$');
    assert.match(new URL(view.path, 'http://localhost').pathname, pattern);
    assert.doesNotMatch(view.path, /[[\]]/);
    assert.ok((view.modes ?? [ 'default' ]).includes('default'));
    for (const state of view.states ?? []) {
      assert.ok((state.modes ?? view.modes ?? [ 'default' ]).includes('default'));
    }
  }
  for (const answer of surface.answers) {
    assert.ok(fs.statSync(path.join(repository, 'tools/ui-check/surfaces', answer.file)).isFile());
  }
}

test(explorerEveryPageHasADeviceView);

function explorerNftInstanceAnswersUseOneStandard() {
  const view = surface.views.find((entry) => entry.name === '/token/[hash]/instance/[id]');
  const readAnswer = (pattern) => {
    const answer = view.answers.find((entry) => entry.pattern === pattern);
    assert.ok(answer, pattern);
    return JSON.parse(fs.readFileSync(path.join(repository, 'tools/ui-check/surfaces', answer.file), 'utf8'));
  };
  const hash = view.path.split('/')[3];
  const id = view.path.split('/')[5];
  const token = readAnswer(`https://explorer.example.test/api/v2/tokens/${hash}`);
  const transfers = readAnswer(`https://explorer.example.test/api/v2/tokens/${hash}/instances/${id}/transfers`);
  const instance = JSON.parse(fs.readFileSync(new URL('./token-instance.json', import.meta.url), 'utf8'));
  assert.equal(token.type, 'ERC-721');
  assert.deepEqual(token, instance.token);
  assert.ok(transfers.items.length > 0);
  for (const transfer of transfers.items) {
    assert.deepEqual(transfer.token, token);
    assert.equal(transfer.total.token_id, instance.id);
    assert.equal(transfer.total.token_instance, null);
    assert.equal('value' in transfer.total, false);
  }
}

test(explorerNftInstanceAnswersUseOneStandard);
