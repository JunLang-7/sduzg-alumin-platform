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
  await page.getByRole('button', { name: /确认/ }).last().click();
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
    await dialog.getByRole('button', { name: /确认/ }).click();
    const keyword = page.getByPlaceholder('姓名、单位、导师');
    await keyword.fill(alumniName);
    await page.getByRole('button', { name: '查询' }).click();
    const row = page.getByRole('row').filter({ hasText: alumniName });
    await expect(row).toBeVisible();
    await row.getByRole('button', { name: '编辑' }).click();
    const editDialog = page.getByRole('dialog', { name: '编辑校友' });
    await editDialog.getByTestId('alumni-grade-input').fill('2027级');
    await editDialog.getByRole('button', { name: /确认/ }).click();
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
    await dialog.getByRole('button', { name: /确认/ }).click();
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
    await page.getByRole('button', { name: /确认/ }).last().click();
    await expect(page.getByText(attachmentName)).toHaveCount(0);
  } finally {
    await removeAlumni(page, alumniName).catch(() => undefined);
  }
});

test('校友上传院史资料，管理员审核后可阅读正式词条', async ({ page }) => {
  const title = `E2E院史词条${Date.now()}`;

  await login(page, '13800001111');
  await page.goto('/history');
  await expect(page.getByRole('heading', { name: '院史共编' })).toBeVisible();
  await page.getByRole('button', { name: '新建词条' }).click();
  await expect(page).toHaveURL(/\/history\/editor\?mode=create$/);

  const editor = page.locator('.history-editor');
  await editor.getByLabel('词条标题').fill(title);
  await editor.getByLabel('正文').fill('用于端到端验证的院史投稿正文');
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
  await expect(page.getByRole('heading', { name: 'history-e2e.pdf' })).toBeVisible();
  await page.getByRole('button', { name: '通过' }).click();
  await page.locator('.ant-modal-confirm-btns .ant-btn-primary').click();
  await expect(page.getByText('处理成功')).toBeVisible();

  await page.evaluate(() => window.localStorage.clear());
  await login(page, '13800001111');
  await page.goto('/history');
  const search = page.getByPlaceholder('搜索词条标题或正文');
  await search.fill(title);
  await search.press('Enter');
  await expect(page.getByText(title).first()).toBeVisible();
});
