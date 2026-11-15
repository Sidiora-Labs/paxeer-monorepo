import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { cp, mkdir, mkdtemp, readFile, rm, stat, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const site = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const committedSource = await readFile(path.join(site, 'src/lib/systems.json'), 'utf8')

async function copySite(t, withInventory = false) {
  const temporary = await mkdtemp(path.join(tmpdir(), 'paxeer-systems-'))
  t.after(() => rm(temporary, { recursive: true, force: true }))
  const root = path.join(temporary, 'docs/site/paxeer-docs')
  const script = path.join(root, 'scripts/generate-systems.mjs')
  const output = path.join(root, 'src/lib/systems.json')
  const inventory = path.join(temporary, 'docs/release/7.0.0-systems.kvx')
  await mkdir(path.dirname(script), { recursive: true })
  await mkdir(path.dirname(output), { recursive: true })
  await cp(path.join(site, 'scripts/generate-systems.mjs'), script)
  await cp(path.join(site, 'src/app'), path.join(root, 'src/app'), { recursive: true })
  await cp(path.join(site, 'src/lib/systems.json'), output)
  if (withInventory) {
    await mkdir(path.dirname(inventory), { recursive: true })
    await cp(path.resolve(site, '../../release/7.0.0-systems.kvx'), inventory)
  }
  return {
    output,
    inventory,
    run: (...args) => spawnSync(process.execPath, [script, ...args], { cwd: temporary, encoding: 'utf8' }),
  }
}

function assertSuccess(result) {
  assert.ifError(result.error)
  assert.equal(result.signal, null)
  assert.equal(result.status, 0, result.stderr)
}

function assertFailure(result, message) {
  assert.ifError(result.error)
  assert.equal(result.signal, null)
  assert.equal(result.status, 1, result.stderr)
  assert.match(result.stderr, message)
}

async function inventoryGeneratesCommittedMap(t) {
  const copy = await copySite(t, true)
  await rm(copy.output)
  const result = copy.run()
  assertSuccess(result)
  assert.match(result.stdout, /^Generated \d+ systems from the 7\.0\.0 inventory\.\n$/)
  assert.equal(await readFile(copy.output, 'utf8'), committedSource)
}
test(inventoryGeneratesCommittedMap)

async function absentInventoryRetainsCommittedMap(t) {
  const copy = await copySite(t)
  const before = await stat(copy.output, { bigint: true })
  for (const args of [[], ['--check']]) {
    const result = copy.run(...args)
    assertSuccess(result)
    assert.equal(result.stderr, '')
    assert.equal(result.stdout, `Using committed src/lib/systems.json (${JSON.parse(committedSource).length} systems); the 7.0.0 inventory is absent.\n`)
    assert.equal(await readFile(copy.output, 'utf8'), committedSource)
    assert.equal((await stat(copy.output, { bigint: true })).mtimeNs, before.mtimeNs)
  }
}
test(absentInventoryRetainsCommittedMap)

async function absentInventoryRejectsInvalidJson(t) {
  const copy = await copySite(t)
  const invalid = committedSource.slice(0, -3)
  await writeFile(copy.output, invalid)
  for (const args of [[], ['--check']]) {
    assertFailure(copy.run(...args), /systems\.json must contain valid JSON/)
    assert.equal(await readFile(copy.output, 'utf8'), invalid)
  }
}
test(absentInventoryRejectsInvalidJson)

async function absentInventoryRejectsEmptyOrNonArrayMap(t) {
  const copy = await copySite(t)
  for (const value of [[], {}, null]) {
    const source = JSON.stringify(value)
    await writeFile(copy.output, source)
    assertFailure(copy.run(), /systems\.json must contain a nonempty array of systems/)
    assert.equal(await readFile(copy.output, 'utf8'), source)
  }
}
test(absentInventoryRejectsEmptyOrNonArrayMap)

async function absentInventoryRequiresSiteFields(t) {
  const copy = await copySite(t)
  const fields = ['id', 'title', 'kind', 'path', 'build', 'artefact', 'configuration', 'health', 'change', 'ports', 'depends', 'docs', 'siteDocs', 'unpublishedDocs']
  for (const field of fields) {
    const systems = JSON.parse(committedSource)
    delete systems[0][field]
    const source = JSON.stringify(systems)
    await writeFile(copy.output, source)
    assertFailure(copy.run(), field === 'id' ? /Invalid system/ : new RegExp(`\\.${field} .*must be`))
    assert.equal(await readFile(copy.output, 'utf8'), source)
  }
}
test(absentInventoryRequiresSiteFields)

async function missingInventoryAndMapNamesBothPaths(t) {
  const copy = await copySite(t)
  await rm(copy.output)
  for (const args of [[], ['--check']]) {
    const result = copy.run(...args)
    assertFailure(result, /Neither inventory .* nor committed system map .* is present/)
    assert.ok(result.stderr.includes(copy.inventory))
    assert.ok(result.stderr.includes(copy.output))
    await assert.rejects(readFile(copy.output), { code: 'ENOENT' })
  }
}
test(missingInventoryAndMapNamesBothPaths)

async function checkDetectsInventoryDriftWithoutWriting(t) {
  const copy = await copySite(t, true)
  const before = await stat(copy.output, { bigint: true })
  assertSuccess(copy.run('--check'))
  assert.equal(await readFile(copy.output, 'utf8'), committedSource)
  assert.equal((await stat(copy.output, { bigint: true })).mtimeNs, before.mtimeNs)
  const inventory = await readFile(copy.inventory, 'utf8')
  const title = JSON.parse(committedSource)[0].title
  const changed = inventory.replace(`title = ${JSON.stringify(title)}`, `title = ${JSON.stringify(title + ' updated')}`)
  assert.notEqual(changed, inventory)
  await writeFile(copy.inventory, changed)
  assertFailure(copy.run('--check'), /differs from .*7\.0\.0-systems\.kvx/)
  assert.equal(await readFile(copy.output, 'utf8'), committedSource)
  assert.equal((await stat(copy.output, { bigint: true })).mtimeNs, before.mtimeNs)
}
test(checkDetectsInventoryDriftWithoutWriting)

async function checkRequiresCommittedMap(t) {
  const copy = await copySite(t, true)
  await rm(copy.output)
  assertFailure(copy.run('--check'), /ENOENT.*systems\.json/)
  await assert.rejects(readFile(copy.output), { code: 'ENOENT' })
}
test(checkRequiresCommittedMap)

async function presentInvalidInventoryIsRejected(t) {
  const copy = await copySite(t, true)
  await writeFile(copy.inventory, '')
  for (const args of [[], ['--check']]) {
    assertFailure(copy.run(...args), /The system inventory must contain systems/)
    assert.equal(await readFile(copy.output, 'utf8'), committedSource)
  }
}
test(presentInvalidInventoryIsRejected)
