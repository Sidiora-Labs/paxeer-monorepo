import { access, mkdir, readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const inventory = path.resolve(root, '../../release/7.0.0-systems.kvx')
const textFields = ['title', 'kind', 'path', 'build', 'artefact', 'configuration', 'health', 'change']
const listFields = ['ports', 'depends', 'docs']
const output = path.join(root, 'src/lib/systems.json')
const check = process.argv.includes('--check')

async function readCommittedSystems() {
  let source
  try {
    source = await readFile(output, 'utf8')
  } catch (error) {
    if (error.code !== 'ENOENT') throw error
    throw new Error(`Neither inventory ${inventory} nor committed system map ${output} is present.`, { cause: error })
  }
  let committed
  try {
    committed = JSON.parse(source)
  } catch (error) {
    throw new Error(`Committed system map ${output} must contain valid JSON.`, { cause: error })
  }
  if (!Array.isArray(committed) || !committed.length) {
    throw new Error(`Committed system map ${output} must contain a nonempty array of systems.`)
  }
  const committedIds = new Set()
  for (const system of committed) {
    if (!system || typeof system !== 'object' || typeof system.id !== 'string' || !/^[a-z0-9-]+$/.test(system.id)) {
      throw new Error(`Invalid system in committed system map ${output}.`)
    }
    if (committedIds.has(system.id)) throw new Error(`Duplicate system ${system.id} in ${output}`)
    committedIds.add(system.id)
    for (const field of textFields) {
      if (typeof system[field] !== 'string' || !system[field].trim()) {
        throw new Error(`${system.id}.${field} in ${output} must be a nonempty string`)
      }
    }
    for (const field of [...listFields, 'siteDocs', 'unpublishedDocs']) {
      if (!Array.isArray(system[field]) || !system[field].every(value => typeof value === 'string' && value.trim())) {
        throw new Error(`${system.id}.${field} in ${output} must be a list of nonempty strings`)
      }
    }
    if (!['new', 'changed', 'unchanged', 'retired'].includes(system.change)) {
      throw new Error(`Unknown release change for ${system.id}: ${system.change}`)
    }
  }
  for (const system of committed) {
    for (const dependency of system.depends) {
      if (!committedIds.has(dependency)) throw new Error(`Unknown dependency ${dependency} for ${system.id} in ${output}`)
    }
  }
  return committed
}

let inventorySource
try {
  inventorySource = await readFile(inventory, 'utf8')
} catch (error) {
  if (error.code !== 'ENOENT') throw error
  const committed = await readCommittedSystems()
  console.log(`Using committed src/lib/systems.json (${committed.length} systems); the 7.0.0 inventory is absent.`)
  process.exit(0)
}

const systems = []
const ids = new Set()
let current

for (const [index, source] of inventorySource.split(/\r?\n/).entries()) {
  const line = source.trim()
  if (!line || line.startsWith('#')) continue
  const section = line.match(/^\[system\.([a-z0-9-]+)\]$/)
  if (section) {
    if (ids.has(section[1])) throw new Error(`Duplicate system ${section[1]}`)
    ids.add(section[1])
    current = { id: section[1] }
    systems.push(current)
    continue
  }
  const field = line.match(/^([a-z]+)\s*=\s*(.+)$/)
  if (!current || !field || ![...textFields, ...listFields].includes(field[1])) {
    throw new Error(`Invalid inventory field at line ${index + 1}`)
  }
  if (Object.hasOwn(current, field[1])) throw new Error(`Duplicate ${current.id}.${field[1]}`)
  current[field[1]] = JSON.parse(field[2])
}

if (!systems.length) throw new Error('The system inventory must contain systems')
for (const system of systems) {
  for (const field of textFields) {
    if (typeof system[field] !== 'string' || !system[field].trim()) {
      throw new Error(`${system.id}.${field} must be a nonempty string`)
    }
  }
  for (const field of listFields) {
    if (!Array.isArray(system[field]) || !system[field].every(value => typeof value === 'string' && value.trim())) {
      throw new Error(`${system.id}.${field} must be a list of nonempty strings`)
    }
  }
  if (!['new', 'changed', 'unchanged', 'retired'].includes(system.change)) {
    throw new Error(`Unknown release change for ${system.id}: ${system.change}`)
  }
  for (const dependency of system.depends) {
    if (!ids.has(dependency)) throw new Error(`Unknown dependency ${dependency} for ${system.id}`)
  }
  const routes = new Set()
  for (const reference of system.docs) {
    const page = reference.match(/^docs\/site\/paxeer-docs\/src\/app\/(.*\/)?page\.tsx$/)
    if (page) routes.add('/' + (page[1] || '').replace(/\/$/, ''))
    else if (reference.startsWith('/') && !reference.startsWith('//')) routes.add(reference)
  }
  system.siteDocs = []
  system.unpublishedDocs = []
  for (const route of routes) {
    const filename = path.resolve(root, 'src/app', '.' + route, 'page.tsx')
    if (!filename.startsWith(path.join(root, 'src/app') + path.sep)) throw new Error(`Invalid site route ${route}`)
    try {
      await access(filename)
      system.siteDocs.push(route)
    } catch (error) {
      if (error.code !== 'ENOENT') throw error
      system.unpublishedDocs.push(route)
      console.warn(`${system.title}: ${route} is an inventory reference without a published site page.`)
    }
  }
}

const generated = JSON.stringify(systems, null, 2) + '\n'
if (check) {
  if (await readFile(output, 'utf8') !== generated) {
    throw new Error(`Committed system map ${output} differs from ${inventory}. Run npm run generate:systems.`)
  }
  console.log(`Checked ${systems.length} systems against the 7.0.0 inventory.`)
} else {
  await mkdir(path.join(root, 'src/lib'), { recursive: true })
  await writeFile(output, generated)
  console.log(`Generated ${systems.length} systems from the 7.0.0 inventory.`)
}
