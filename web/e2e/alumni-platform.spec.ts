import { expect, test, type Page } from '@playwright/test';

const password = 'Admin@123456';
const attachmentContent = Buffer.from('%PDF-1.4\nE2E history attachment\n%%EOF\n');

async function login(page: Page, account: string) {
  await page.goto('/login');
  await page.getByPlaceholder('手机、邮箱或账号').fill(account);
  await page.getByLabel('密码', { exact: true }).fill(password);
  await page.getByRole('button', { name: '登录' }).click();
  await page.waitForURL((url) => url.pathname !== '/login');
}

test('未登录校友不能访问院史共编', async ({ page }) => {
  await page.goto('/history');
  await expect(page).toHaveURL(/\/login$/);
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
