import { expect, test, type Page } from '@playwright/test';
import { readFile } from 'node:fs/promises';

const password = 'Admin@123456';
const attachmentContent = Buffer.from('%PDF-1.4\nE2E history attachment\n%%EOF\n');

async function login(page: Page, account: string) {
  await page.goto('/login');
  await page.locator('input[autocomplete="username"]').fill(account);
  await page.locator('input[autocomplete="current-password"]').fill(password);
  await page.locator('button[type="submit"]').click();
  await page.waitForURL((url) => url.pathname !== '/login');
}

async function removeAlumni(page: Page, name: string) {
  await page.goto('/admin/alumni');
  const keyword = page.getByPlaceholder('姓名、单位、导师');
  await keyword.fill(name);
  await page.getByRole('button', { name: '查询' }).click();
  const row = page.getByRole('row').filter({ hasText: name });
  if (!(await row.count())) return;
  await row.getByRole('button', { name: '删除' }).click();
  await page
    .getByRole('button', { name: /确\s*定/ })
    .last()
    .click();
}

test('未登录校友不能访问院史共编', async ({ page }) => {
  await page.goto('/history');
  await expect(page).toHaveURL(/\/login$/);
});

test('普通校友不能进入管理员功能', async ({ page }) => {
  await login(page, '13800001111');
  await page.goto('/admin/alumni');
  await expect(page).toHaveURL(/\/403$/);
});

test('校友令牌不能查询通用档案，但本人资料和院史仍可访问', async ({ page }) => {
  for (const path of ['/api/v1/alumni', '/api/v1/alumni/1']) {
    const response = await page.request.get(path);
    expect(response.status()).toBe(401);
  }

  await login(page, '13800001111');
  const token = await page.evaluate(() => window.localStorage.getItem('sdu_alumni_access_token'));
  expect(token).toBeTruthy();
  const headers = { Authorization: `Bearer ${token}` };
  const meResponse = await page.request.get('/api/v1/alumni/me', { headers });
  expect(meResponse.status()).toBe(200);
  const ownProfile = (await meResponse.json()).data;
  expect(ownProfile.id).toBeGreaterThan(0);

  for (const path of [
    '/api/v1/alumni',
    `/api/v1/alumni/${ownProfile.id}`,
    '/api/v1/alumni/0',
    '/api/v1/alumni/not-a-number',
  ]) {
    const response = await page.request.get(path, { headers });
    expect(response.status()).toBe(403);
    const body = await response.json();
    expect(body.code).toBe(40300);
    expect(body).not.toHaveProperty('data');
  }
  const historyResponse = await page.request.get('/api/v1/history/entries', { headers });
  expect(historyResponse.status()).toBe(200);
});

test('管理员可以创建、编辑并清理校友档案', async ({ page }) => {
  const alumniName = `E2E校友${Date.now()}`;
  await login(page, 'admin');
  try {
    await page.getByRole('button', { name: '新增校友' }).click();
    const dialog = page.getByRole('dialog', { name: '新增校友' });
    await dialog.getByTestId('alumni-name-input').fill(alumniName);
    await dialog.getByTestId('alumni-grade-input').fill('2026级');
    await dialog.getByRole('combobox').first().click();
    await page.getByText('MPA专业学位研究生', { exact: true }).last().click();
    await dialog.getByRole('button', { name: /确\s*定/ }).click();
    const keyword = page.getByPlaceholder('姓名、单位、导师');
    await keyword.fill(alumniName);
    await page.getByRole('button', { name: '查询' }).click();
    const row = page.getByRole('row').filter({ hasText: alumniName });
    await expect(row).toBeVisible();
    await row.getByRole('button', { name: '编辑' }).click();
    const editDialog = page.getByRole('dialog', { name: '编辑校友' });
    await editDialog.getByTestId('alumni-grade-input').fill('2027级');
    await editDialog.getByRole('button', { name: /确\s*定/ }).click();
    await page.reload();
    await expect(page.getByRole('row').filter({ hasText: alumniName })).toContainText('2027级');
  } finally {
    await removeAlumni(page, alumniName).catch(() => undefined);
  }
});

test('管理员可以上传、下载并删除校友附件', async ({ page }) => {
  const alumniName = `E2E附件校友${Date.now()}`;
  const attachmentName = 'e2e-alumni-attachment.pdf';
  await login(page, 'admin');
  try {
    await page.getByRole('button', { name: '新增校友' }).click();
    const dialog = page.getByRole('dialog', { name: '新增校友' });
    await dialog.getByTestId('alumni-name-input').fill(alumniName);
    await dialog.getByTestId('alumni-grade-input').fill('2026级');
    await dialog.getByRole('combobox').first().click();
    await page.getByText('MPA专业学位研究生', { exact: true }).last().click();
    await dialog.getByRole('button', { name: /确\s*定/ }).click();
    const keyword = page.getByPlaceholder('姓名、单位、导师');
    await keyword.fill(alumniName);
    await page.getByRole('button', { name: '查询' }).click();
    await page
      .getByRole('row')
      .filter({ hasText: alumniName })
      .getByRole('button', { name: '查看' })
      .click();
    await page
      .getByTestId('alumni-file-upload-degree_archive')
      .locator('input[type="file"]')
      .setInputFiles({
        name: attachmentName,
        mimeType: 'application/pdf',
        buffer: attachmentContent,
      });
    const fileRow = page.getByRole('listitem').filter({ hasText: attachmentName });
    await expect(fileRow).toBeVisible();
    const downloadPromise = page.waitForEvent('download');
    await fileRow.getByRole('button', { name: '下载' }).click();
    const download = await downloadPromise;
    expect(await readFile((await download.path())!)).toEqual(attachmentContent);
    await fileRow.getByRole('button', { name: '删除' }).click();
    await page
      .getByRole('button', { name: /确\s*定/ })
      .last()
      .click();
    await expect(page.getByText(attachmentName)).toHaveCount(0);
  } finally {
    await removeAlumni(page, alumniName).catch(() => undefined);
  }
});

test('正文工具栏、快捷键和预览可用于编辑 Markdown', async ({ page }) => {
  await login(page, '13800001111');
  await page.goto('/history/editor?mode=create');
  const editor = page.locator('.history-editor');
  const textarea = editor.getByLabel('正文', { exact: true });
  await expect(textarea).toHaveAttribute('placeholder', '请输入正文');
  await expect(editor.locator('.markdown-editor__hint')).toHaveCount(0);
  await expect(editor.locator('.ant-form-item-extra')).toHaveCount(0);
  await textarea.fill('学科简介');
  await textarea.press('ControlOrMeta+a');
  await editor.getByRole('button', { name: '加粗', exact: true }).click();
  await expect(textarea).toHaveValue('**学科简介**');
  await textarea.press('ControlOrMeta+i');
  await expect(textarea).toHaveValue('***学科简介***');
  await editor.getByRole('button', { name: '插入标题' }).click();
  await page.getByRole('menuitem', { name: '四级标题 H4' }).click();
  await expect(textarea).toHaveValue('#### ***学科简介***');
  await editor.getByRole('tab', { name: '预览', exact: true }).click();
  const preview = editor.getByRole('tabpanel', { name: '预览', exact: true });
  await expect(preview.getByRole('heading', { name: '学科简介', level: 4 })).toBeVisible();
  await expect(preview.locator('strong')).toHaveText('学科简介');
  await expect(preview.locator('em')).toHaveText('学科简介');
  await editor.getByRole('tab', { name: '编辑', exact: true }).click();
  await textarea.focus();
  await textarea.press('ControlOrMeta+Alt+6');
  await expect(textarea).toHaveValue('###### ***学科简介***');
  await textarea.fill('学院网站');
  await textarea.press('ControlOrMeta+a');
  await textarea.press('ControlOrMeta+k');
  await expect(textarea).toHaveValue('[学院网站](https://example.com)');
  await textarea.pressSequentially('https://www.sdu.edu.cn');
  await editor.getByRole('tab', { name: '预览', exact: true }).click();
  await expect(preview.getByRole('link', { name: '学院网站' })).toHaveAttribute(
    'href',
    'https://www.sdu.edu.cn',
  );
  await editor.getByRole('tab', { name: '编辑', exact: true }).click();
  await page.screenshot({ path: 'test-results/markdown-editor-desktop.png', fullPage: true });
  await page.setViewportSize({ width: 375, height: 812 });
  await expect(editor.getByRole('button', { name: '任务列表', exact: true })).toBeVisible();
  const fits = await editor
    .locator('.markdown-editor')
    .evaluate((element) => element.scrollWidth <= element.clientWidth);
  expect(fits).toBe(true);
  await page.screenshot({ path: 'test-results/markdown-editor-mobile.png', fullPage: true });
});

test('校友上传院史资料，管理员审核后可阅读正式词条', async ({ page }) => {
  test.setTimeout(90_000);
  const title = `E2E院史词条${Date.now()}`;

  await login(page, '13800001111');
  await page.goto('/history');
  await expect(page.getByRole('heading', { name: '院史共编' })).toBeVisible();
  await page.getByRole('button', { name: '新建词条' }).click();
  await expect(page).toHaveURL(/\/history\/editor\?mode=create(?:&entryId=\d+)?$/);

  const editor = page.locator('.history-editor');
  await editor.getByLabel('词条标题').fill(title);
  await editor.getByLabel('正文', { exact: true }).fill('用于端到端验证的院史投稿正文');
  await editor.getByLabel('资料来源').fill('E2E 测试资料来源');
  await editor.locator('input[type="file"]').setInputFiles({
    name: 'history-e2e.pdf',
    mimeType: 'application/pdf',
    buffer: attachmentContent,
  });
  await editor.getByText('我确认附件来源真实且有权提交').click();
  await editor.getByRole('button', { name: '提交审核' }).click();
  await expect(page).toHaveURL(/\/history$/);
  await expect(page.getByText(title).last()).toBeVisible();

  await page.evaluate(() => window.localStorage.clear());
  await login(page, 'admin');
  await page.goto('/admin/history/reviews');
  const row = page.getByRole('row').filter({ hasText: title });
  await expect(row).toBeVisible();
  await row.getByRole('button', { name: '审核' }).click();
  await expect(page).toHaveURL(/\/admin\/history\/reviews\/\d+$/);
  await expect(page.getByRole('heading', { name: '词条正文' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'history-e2e.pdf' })).toBeVisible();
  await page.getByRole('button', { name: '通过' }).click();
  await page.locator('.ant-modal-confirm-btns .ant-btn-primary').click();
  await expect(page.getByText('处理成功')).toBeVisible();

  const headingTitles = ['一级概况', '二级沿革', '三级学科', '四级方向', '五级成果', '六级资料'];
  const revisedBody = [
    '管理员修改后应在投稿预览中同步显示的新正文',
    ...headingTitles.map((heading, index) => `${'#'.repeat(index + 1)} ${heading}\n章节正文`),
  ].join('\n');
  await page.goto('/history');
  const adminSearch = page.getByPlaceholder('搜索正式词条的标题、简介或正文');
  await adminSearch.fill(title);
  await adminSearch.press('Enter');
  await expect(page.locator('.history-page__content article > h2')).toHaveText(title);
  await page.getByRole('button', { name: '编辑词条' }).click();
  await page.locator('.history-editor').getByLabel('正文', { exact: true }).fill(revisedBody);
  await page.getByRole('button', { name: '保存更改' }).click();
  await expect(page).toHaveURL(/\/history$/);

  await page.evaluate(() => window.localStorage.clear());
  await login(page, '13800001111');
  await page.goto('/history');
  const search = page.getByPlaceholder('搜索正式词条的标题、简介或正文');
  await search.fill(title);
  await search.press('Enter');
  await expect(page.getByText(title).first()).toBeVisible();
  const toc = page.getByRole('navigation', { name: '本页目录' });
  for (const [index, heading] of headingTitles.entries()) {
    const section = page.locator('.history-page__body').getByRole('heading', {
      name: heading,
      level: index + 1,
      exact: true,
    });
    await expect(section).toHaveAttribute('id', heading);
    const link = toc.getByRole('button', { name: heading, exact: true });
    await expect(link).toHaveCSS('padding-left', `${10 + index * 10}px`);
    await link.click();
    await expect(section).toBeInViewport();
  }
  await page.locator('.history-page__mine-item').filter({ hasText: title }).click();
  const details = page.getByRole('dialog', { name: '投稿详情' });
  await expect(details.getByText('当前正式正文', { exact: true })).toHaveCount(0);
  await expect(details.getByText(/当前正式版本\s*v\d+/)).toHaveCount(0);
  await expect(details.locator('.history-page__contribution-preview > span')).toHaveText(
    revisedBody,
  );
  await details.getByRole('button', { name: '查看详情' }).click();
  const fullContent = page.getByRole('dialog', { name: '正文详情', exact: true });
  await expect(fullContent.getByText('管理员修改后应在投稿预览中同步显示的新正文')).toBeVisible();
  for (const [index, heading] of headingTitles.entries()) {
    await expect(
      fullContent.getByRole('heading', { name: heading, level: index + 1 }),
    ).toBeVisible();
  }
  await fullContent.locator('.ant-modal-close').click();
  await details.locator('.ant-modal-close').click();
  await expect(page.getByRole('button', { name: '删除词条', exact: true })).toHaveCount(0);
  const alumniToken = await page.evaluate(() =>
    window.localStorage.getItem('sdu_alumni_access_token'),
  );
  const alumniHeaders = { Authorization: `Bearer ${alumniToken}` };
  const entryResponse = await page.request.get(
    `/api/v1/history/entries?keyword=${encodeURIComponent(title)}`,
    { headers: alumniHeaders },
  );
  const entryID = (await entryResponse.json()).data[0].id;
  const forbidden = await page.request.delete(`/api/v1/history/entries/${entryID}`, {
    headers: alumniHeaders,
  });
  expect(forbidden.status()).toBe(403);
  await expect(page.getByRole('button', { name: '申请删除词条', exact: true })).toHaveCount(0);
  const forbiddenRequest = await page.request.post(
    `/api/v1/history/entries/${entryID}/deletion-requests`,
    {
      headers: alumniHeaders,
      data: { reason: '校友不能申请删除' },
    },
  );
  expect(forbiddenRequest.status()).toBe(403);
  await page.evaluate(() => window.localStorage.clear());
  await login(page, 'admin');
  await page.goto('/history');
  await page.getByRole('button', { name: '申请删除词条', exact: true }).click();
  const deletionDialog = page.getByRole('dialog', { name: '申请删除词条', exact: true });
  await deletionDialog.getByLabel('删除原因').fill('原投稿人申请删除测试词条');
  await deletionDialog.getByRole('button', { name: '提交申请', exact: true }).click();
  await expect(page.getByRole('button', { name: '删除申请待审核' })).toBeDisabled();
  await expect(page.locator('.history-page__content article > h2')).toHaveText(title);

  await page.evaluate(() => window.localStorage.clear());
  await login(page, 'admin');
  await page.goto('/admin/history/reviews');
  const requestRow = page.getByRole('row').filter({ hasText: '原投稿人申请删除测试词条' });
  await requestRow.getByRole('button', { name: /驳\s*回/ }).click();
  const rejectDialog = page.getByRole('dialog', { name: '驳回申请' });
  await rejectDialog.getByLabel('驳回原因').fill('资料仍有参考价值，请补充原因');
  await rejectDialog.getByRole('button', { name: /驳\s*回/ }).click();
  await expect(requestRow).toHaveCount(0);

  await page.evaluate(() => window.localStorage.clear());
  await login(page, 'admin');
  await page.goto('/history');
  await expect(page.getByText('资料仍有参考价值，请补充原因', { exact: false })).toBeVisible();
  await page.getByRole('button', { name: '申请删除词条', exact: true }).click();
  await deletionDialog.getByLabel('删除原因').fill('再次申请删除，确认测试数据应撤下');
  await deletionDialog.getByRole('button', { name: '提交申请', exact: true }).click();
  // 等待申请落库后再切换账号，否则在途请求会被导航取消。
  await expect(page.getByRole('button', { name: '删除申请待审核' })).toBeDisabled();

  await page.evaluate(() => window.localStorage.clear());
  await login(page, 'admin');
  await page.goto('/admin/history/reviews');
  const approvedRow = page.getByRole('row').filter({ hasText: '再次申请删除，确认测试数据应撤下' });
  await approvedRow.getByRole('button', { name: /同\s*意/ }).click();
  await page
    .getByRole('dialog', { name: '同意删除？' })
    .getByRole('button', { name: /同\s*意/ })
    .click();
  await expect(approvedRow).toHaveCount(0);

  await page.evaluate(() => window.localStorage.clear());
  await login(page, '13800001111');
  await page.goto('/history');
  await expect(page.locator('.history-page__content article > h2')).toHaveText('暂无已发布词条');
  await expect(page.getByText('我的删除申请', { exact: true })).toHaveCount(0);
  await page.locator('.history-page__mine-item').filter({ hasText: title }).click();
  await expect(details.locator('.history-page__contribution-preview > span')).toHaveText(
    '用于端到端验证的院史投稿正文',
  );
  await expect(details.getByText('history-e2e.pdf')).toBeVisible();
  await details.locator('.ant-modal-close').click();

  // A separate entry exercises administrator direct deletion and cancellation.
  const token = await page.evaluate(() => window.localStorage.getItem('sdu_alumni_access_token'));
  const headers = { Authorization: `Bearer ${token}` };
  const created = await page.request.post('/api/v1/history/contributions', {
    headers,
    data: { title: `${title}-direct`, content: '直接删除测试正文', source_note: 'E2E' },
  });
  expect(created.status()).toBe(201);
  const contributionID = (await created.json()).data.id;
  expect(
    (
      await page.request.post(`/api/v1/history/contributions/${contributionID}/submit`, { headers })
    ).ok(),
  ).toBe(true);
  await page.evaluate(() => window.localStorage.clear());
  await login(page, 'admin');
  const adminToken = await page.evaluate(() =>
    window.localStorage.getItem('sdu_alumni_access_token'),
  );
  const reviewed = await page.request.post(`/api/v1/history/reviews/${contributionID}`, {
    headers: { Authorization: `Bearer ${adminToken}` },
    data: { action: 'approve', review_comment: '' },
  });
  expect(reviewed.ok()).toBe(true);
  await page.goto('/history');
  await expect(page.locator('.history-page__content article > h2')).toHaveText(`${title}-direct`);
  const deleteButton = page
    .locator('.history-page__meta')
    .getByRole('button', { name: '删除词条', exact: true });
  await deleteButton.click();
  const directDialog = page.getByRole('dialog').filter({ hasText: `删除词条“${title}-direct”？` });
  await directDialog.getByRole('button', { name: /取\s*消/ }).click();
  await expect(directDialog).toHaveCount(0);
  await expect(page.locator('.history-page__content article > h2')).toHaveText(`${title}-direct`);
  const entryURL = '**/api/v1/history/entries/*';
  await page.route(entryURL, async (route) => {
    if (route.request().method() === 'DELETE')
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({ code: 50300, message: '测试删除失败', data: null }),
      });
    else await route.continue();
  });
  await deleteButton.click();
  await directDialog.getByRole('button', { name: '删除词条', exact: true }).click();
  await expect(page.getByText('测试删除失败', { exact: true })).toBeVisible();
  await expect(page.locator('.history-page__content article > h2')).toHaveText(`${title}-direct`);
  await directDialog.getByRole('button', { name: /取\s*消/ }).click();
  await expect(directDialog).toHaveCount(0);
  await page.unroute(entryURL);
  await deleteButton.click();
  await directDialog.getByRole('button', { name: '删除词条', exact: true }).click();
  await expect(page.locator('.history-page__content article > h2')).toHaveText('暂无已发布词条');
});
