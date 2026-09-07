import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const scriptDir = dirname(fileURLToPath(import.meta.url));
const detailSource = await readFile(
  resolve(scriptDir, '../src/pages/alumni/AlumniDetailPage.tsx'),
  'utf8',
);
const historySource = await readFile(
  resolve(scriptDir, '../src/components/AuditOperationHistoryCard.tsx'),
  'utf8',
);

test('alumni detail keeps profile before operation history', () => {
  const profilePosition = detailSource.indexOf('title="校友档案"');
  const historyPosition = detailSource.indexOf('<AuditOperationHistoryCard');
  assert.notEqual(profilePosition, -1, 'alumni profile section is required');
  assert.notEqual(historyPosition, -1, 'operation history section is required');
  assert.ok(
    profilePosition < historyPosition,
    'operation history should follow the alumni profile',
  );
});

test('alumni operation history is a read-only linked record list', () => {
  assert.match(historySource, /title: '时间'/);
  assert.match(historySource, /title: '操作人'/);
  assert.match(historySource, /title: '管理范围'/);
  assert.match(historySource, /title: '类型'/);
  assert.match(historySource, /详情/);
  assert.match(historySource, /auditApi\.list/);
  assert.match(historySource, /auditApi\.detail/);
  assert.match(historySource, /pagination=\{false\}/);
  assert.doesNotMatch(historySource, /撤销/);
  assert.doesNotMatch(historySource, /onEdit|handleEdit|编辑/);
});
