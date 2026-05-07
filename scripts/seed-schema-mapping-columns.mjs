import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { spawnSync } from 'node:child_process'

const seedPath = resolve(process.env.HAOHAO_SCHEMA_MAPPING_SEED ?? 'samples/schema-mapping/invoice-columns.json')
const tenantSlug = process.env.HAOHAO_TENANT_SLUG ?? 'acme'
const databaseUrl = process.env.DATABASE_URL ?? readDatabaseURLFromEnv() ?? 'postgres://haohao:haohao@127.0.0.1:5432/haohao?sslmode=disable'
const psql = process.env.PSQL ?? findPsql()

function readDatabaseURLFromEnv() {
  try {
    const env = readFileSync('.env', 'utf8')
    for (const line of env.split(/\r?\n/)) {
      const trimmed = line.trim()
      if (!trimmed || trimmed.startsWith('#')) {
        continue
      }
      const index = trimmed.indexOf('=')
      if (index <= 0) {
        continue
      }
      const key = trimmed.slice(0, index).trim()
      if (key !== 'DATABASE_URL') {
        continue
      }
      return trimmed.slice(index + 1).trim().replace(/^['"]|['"]$/g, '')
    }
  } catch {
    return undefined
  }
  return undefined
}

function findPsql() {
  for (const candidate of ['psql-18', 'psql']) {
    const result = spawnSync(candidate, ['--version'], { encoding: 'utf8', stdio: ['ignore', 'ignore', 'ignore'] })
    if (result.status === 0) {
      return candidate
    }
  }
  return 'psql'
}

function sqlString(value) {
  return `'${String(value ?? '').replaceAll("'", "''")}'`
}

function sqlJSON(value) {
  return `${sqlString(JSON.stringify(value ?? []))}::jsonb`
}

function validateSeed(seed) {
  if (!seed || typeof seed !== 'object') {
    throw new Error('seed file must contain an object')
  }
  for (const field of ['domain', 'schemaType', 'columns']) {
    if (!(field in seed)) {
      throw new Error(`seed file is missing ${field}`)
    }
  }
  if (!Array.isArray(seed.columns) || seed.columns.length === 0) {
    throw new Error('seed columns must contain at least one item')
  }
  for (const [index, column] of seed.columns.entries()) {
    for (const field of ['targetColumn', 'description']) {
      if (!String(column[field] ?? '').trim()) {
        throw new Error(`columns[${index}].${field} is required`)
      }
    }
  }
}

const seed = JSON.parse(readFileSync(seedPath, 'utf8'))
validateSeed(seed)

const language = seed.language ?? 'ja'
const version = Number.isInteger(seed.version) ? seed.version : 1
const statements = [
  '\\set ON_ERROR_STOP on',
  'BEGIN;',
  `DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM tenants WHERE slug = ${sqlString(tenantSlug)}) THEN
    RAISE EXCEPTION 'tenant not found: ${tenantSlug.replaceAll("'", "''")}';
  END IF;
END $$;`,
]

for (const column of seed.columns) {
  statements.push(`WITH target_tenant AS (SELECT id FROM tenants WHERE slug = ${sqlString(tenantSlug)} LIMIT 1)
INSERT INTO data_pipeline_schema_columns (
  tenant_id,
  domain,
  schema_type,
  target_column,
  description,
  aliases,
  examples,
  language,
  version
)
SELECT
  id,
  ${sqlString(seed.domain)},
  ${sqlString(seed.schemaType)},
  ${sqlString(column.targetColumn)},
  ${sqlString(column.description)},
  ${sqlJSON(column.aliases)},
  ${sqlJSON(column.examples)},
  ${sqlString(column.language ?? language)},
  ${version}
FROM target_tenant
ON CONFLICT (tenant_id, domain, schema_type, target_column, version) DO UPDATE
SET
  description = EXCLUDED.description,
  aliases = EXCLUDED.aliases,
  examples = EXCLUDED.examples,
  language = EXCLUDED.language,
  archived_at = NULL,
  updated_at = now();`)
}

statements.push('COMMIT;')
statements.push(`SELECT count(*) AS seeded_schema_columns
FROM data_pipeline_schema_columns c
JOIN tenants t ON t.id = c.tenant_id
WHERE t.slug = ${sqlString(tenantSlug)}
  AND c.domain = ${sqlString(seed.domain)}
  AND c.schema_type = ${sqlString(seed.schemaType)}
  AND c.version = ${version};`)

const result = spawnSync(psql, [databaseUrl], {
  input: statements.join('\n\n'),
  encoding: 'utf8',
  stdio: ['pipe', 'pipe', 'pipe'],
})

if (result.stdout) {
  process.stdout.write(result.stdout)
}
if (result.stderr) {
  process.stderr.write(result.stderr)
}
if (result.status !== 0) {
  process.exit(result.status ?? 1)
}
