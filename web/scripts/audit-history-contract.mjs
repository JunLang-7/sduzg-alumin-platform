import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const scriptDir = dirname(fileURLToPath(import.meta.url));
const pageSource = await readFile(
  resolve(scriptDir, '../src/pages/admin/AuditHistoryPage.tsx'),
  'utf8',
);
const apiSource = await readFile(resolve(scriptDir, '../src/api/audit.ts'), 'utf8');
const typeSource = await readFile(resolve(scriptDir, '../src/types/audit.ts'), 'utf8');

test('audit history page keeps the list-level review contract', () => {
  assert.match(pageSource, /title: '时间'/);
  assert.match(pageSource, /title: '操作人'/);
  assert.match(pageSource, /title: '管理范围'/);
  assert.match(pageSource, /title: '校友档案'/);
  assert.match(pageSource, /title: '类型'/);
  assert.match(pageSource, /详情/);
  assert.match(pageSource, /DatePicker\.RangePicker/);
  assert.match(pageSource, /className="audit-scope-select"/);
  assert.match(pageSource, /className="audit-action-select"/);
  assert.match(pageSource, /record\.target_id/);
  assert.match(pageSource, /`\/alumni\/\$\{record\.target_id\}`/);
});

test('audit history frontend exposes list and detail API contracts', () => {
  assert.match(apiSource, /auditApi/);
  assert.match(apiSource, /\/admin\/audit\/operations/);
  assert.match(apiSource, /detail\(id/);
  assert.match(typeSource, /AUDIT_ACTION_OPTIONS/);
  assert.match(typeSource, /AUDIT_SCOPE_TAGS/);
});
