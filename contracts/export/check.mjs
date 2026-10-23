// SPDX-License-Identifier: AGPL-3.0-only
// Check a proposed contract against the immutable, independently accepted producer.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { resolve, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

const directory = dirname(fileURLToPath(import.meta.url));
const arguments_ = process.argv.slice(2);
assert.equal(arguments_.length, 2, 'Usage: node contracts/export/check.mjs --web-checkout <checkout>');
assert.equal(arguments_[0], '--web-checkout');
const web = resolve(arguments_[1]);
const requireWeb = createRequire(join(web, 'package.json'));
const lock = JSON.parse(readFileSync(join(directory, 'source-lock.json')));
const sha = (data) => createHash('sha256').update(data).digest('hex');
const source = (path) => execFileSync('git', ['show', `${lock.source_commit}:${path}`], { cwd: web });
for (const [path, hash] of Object.entries(lock.source_files))
  assert.equal(sha(source(path)), hash, `Immutable source mismatch: ${path}`);
assert.equal(
  sha(execFileSync('git', ['show', `${lock.compatible_f5_commit}:src/lib/export.ts`], { cwd: web })),
  lock.source_files['src/lib/export.ts'],
  'The accepted F5 producer changed the accepted F6 format',
);
for (const tool of lock.validation_tools)
  assert.equal(requireWeb(`${tool.package}/package.json`).version, tool.version);
for (const notice of lock.notices) {
  assert.equal(sha(readFileSync(join(directory, notice.file))), notice.sha256);
  assert.equal(
    sha(readFileSync(join(web, 'node_modules', notice.package, notice.package_file))),
    notice.sha256,
    `Original tool notice mismatch: ${notice.package}/${notice.package_file}`,
  );
}
const schemaBytes = readFileSync(join(directory, 'result-v1.schema.json'));
assert.equal(sha(schemaBytes), lock.schema_sha256);
const Ajv2020 = requireWeb('ajv/dist/2020.js').default;
const ajv = new Ajv2020({ strict: true, allErrors: true });
const schema = JSON.parse(schemaBytes);
assert.equal(ajv.validateSchema(schema), true);
const validate = ajv.compile(schema);
const typescript = requireWeb('typescript');
const compiled = typescript.transpileModule(source('src/lib/export.ts').toString('utf8'), {
  compilerOptions: { module: typescript.ModuleKind.CommonJS, target: typescript.ScriptTarget.ES2022 },
}).outputText;
const producer = { exports: {} };
new Function('exports', 'require', 'module', compiled)(producer.exports, requireWeb, producer);
const { encodeExport, MAX_EXPORT_BYTES } = producer.exports;
assert.equal(MAX_EXPORT_BYTES, lock.max_export_bytes);
const decoder = new TextDecoder('utf-8', { fatal: true });
function semanticStatus(bytes) {
  if (bytes.byteLength > lock.max_export_bytes) return 'byte_limit';
  let data;
  try { data = JSON.parse(decoder.decode(bytes)); }
  catch { return 'encoding_or_json'; }
  if (!validate(data)) return 'schema';
  if (data.rows.some((row) => row.length !== data.columns.length)) return 'row_width';
  return 'valid';
}
const cases = JSON.parse(readFileSync(join(directory, 'fixtures/vectors.json')));
const goldens = new Map();
let csvPositive = 0;
let csvNegative = 0;
for (const vector of cases) {
  const expected = JSON.parse(readFileSync(join(directory, 'fixtures/expected', `${vector.id}.json`)));
  goldens.set(vector.id, expected);
  const output = encodeExport(vector.result, vector.scope, 'json');
  assert.equal(semanticStatus(output.bytes), 'valid', vector.id);
  assert.deepEqual(JSON.parse(decoder.decode(output.bytes)), expected, vector.id);
  assert.equal(output.filename, 'db-manager-result.json');
  assert.equal(output.mime, 'application/json;charset=utf-8');
  for (const secret of ['synthetic-sql-must-not-export', 'synthetic-token-must-not-export', 'synthetic-password-must-not-export'])
    assert.equal(decoder.decode(output.bytes).includes(secret), false, `${vector.id}: whitelist`);
  for (const format of ['csv-raw', 'csv-spreadsheet']) {
    if (vector.csv === 'rejected-controls') {
      assert.throws(() => encodeExport(vector.result, vector.scope, format), /JSON/);
      csvNegative++;
    } else {
      const csv = encodeExport(vector.result, vector.scope, format);
      assert.deepEqual(Buffer.from(csv.bytes), readFileSync(join(directory, 'fixtures/expected', `${vector.id}.${format}.csv`)), `${vector.id}/${format}`);
      assert.equal(csv.filename, 'db-manager-result.csv');
      assert.equal(csv.mime, 'text/csv;charset=utf-8');
      csvPositive++;
    }
  }
}
const negatives = JSON.parse(readFileSync(join(directory, 'fixtures/negative-cases.json')));
for (const vector of negatives) {
  const data = structuredClone(goldens.get(vector.base));
  let target = data;
  for (const part of vector.path.slice(0, -1)) target = target[part];
  const key = vector.path.at(-1);
  if (vector.operation === 'delete') delete target[key];
  else target[key] = vector.value;
  assert.equal(semanticStatus(Buffer.from(JSON.stringify(data))), vector.expected, vector.id);
}
assert.equal(semanticStatus(Buffer.from([0xff])), 'encoding_or_json');
assert.equal(semanticStatus(Buffer.from('{')), 'encoding_or_json');
let rejectedControls = 0;
for (const code of [...Array.from({ length: 32 }, (_, n) => n).filter((n) => n !== 10 && n !== 13), ...Array.from({ length: 33 }, (_, n) => 127 + n)]) {
  for (const field of ['header', 'cell']) {
    const data = { columns: [{ name: 'value', type: 'TEXT' }], rows: [[{ kind: 'text', value: 'value' }]], truncated: false };
    if (field === 'header') data.columns[0].name = String.fromCharCode(code);
    else data.rows[0][0].value = String.fromCharCode(code);
    const encoded = encodeExport(data, { kind: 'sql_result' }, 'json');
    assert.equal(semanticStatus(encoded.bytes), 'valid');
    assert.deepEqual(JSON.parse(decoder.decode(encoded.bytes)).rows, data.rows);
    for (const format of ['csv-raw', 'csv-spreadsheet']) {
      assert.throws(() => encodeExport(data, { kind: 'sql_result' }, format), /JSON/);
      rejectedControls++;
    }
  }
}
for (const format of ['json', 'csv-raw', 'csv-spreadsheet']) {
  const input = { columns: [{ name: 'value', type: '' }], rows: [[{ kind: 'text', value: '' }]], truncated: false };
  const overhead = encodeExport(input, { kind: 'sql_result' }, format).bytes.byteLength;
  input.rows[0][0].value = 'x'.repeat(MAX_EXPORT_BYTES - overhead);
  const exact = encodeExport(input, { kind: 'sql_result' }, format);
  assert.equal(exact.bytes.byteLength, MAX_EXPORT_BYTES, `${format}: inclusive final limit`);
  if (format === 'json') {
    assert.equal(semanticStatus(exact.bytes), 'valid');
    assert.equal(semanticStatus(Buffer.concat([Buffer.from(exact.bytes), Buffer.from(' ')])), 'byte_limit');
  }
  input.rows[0][0].value += 'ä';
  assert.throws(() => encodeExport(input, { kind: 'sql_result' }, format), /8 MiB/);
}
console.log(`C-Export1: schema, ${cases.length} JSON fixtures, ${csvPositive} CSV byte fixtures, ${negatives.length + 2} negatives, ${rejectedControls + csvNegative} CSV control rejections and 3 exact-byte boundaries passed.`);
console.log('Immutable F6/F5 producer bytes, tool versions and 3 original tool notice hashes passed. No native/app/OS proof is implied.');
