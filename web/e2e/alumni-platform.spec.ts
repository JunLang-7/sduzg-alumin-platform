import { expect, test, type Page } from '@playwright/test';
import { readFile } from 'node:fs/promises';

const password = 'Admin@123456';
const attachmentName = 'e2e-attachment.pdf';
const attachmentContent = Buffer.from('%PDF-1.4\nE2E attachment verification payload\n%%EOF\n');

async function login(page: Page, account: string) {
  await page.goto('/login');
  await page.getByPlaceholder('手机号/邮箱/账号').fill(account);
  await page.getByRole('textbox', { name: '* 密码', exact: true }).fill(password);
  await page.getByRole('button', { name: '登录' }).click();
}

async function removeAlumni(page: Page, name: string) {
  await page.goto('/admin/alumni');
  const keyword = page.getByPlaceholder('姓名、单位、导师');
  await keyword.fill(name);
  await page.getByRole('button', { name: '查询' }).click();
  const row = page.getByRole('row').filter({ hasText: name });
  if ((await row.count()) === 0) return;
  await row.getByRole('button', { name: '删除' }).click();
  await page
    .getByRole('button', { name: /确\s*定/ })
    .last()
    .click();
  await expect(page.getByText('校友档案已删除')).toBeVisible();
}

test('未登录和普通校友不能进入管理员功能', async ({ page }) => {
  await page.goto('/admin/alumni');
  await expect(page).toHaveURL(/\/login$/);

  await login(page, '13800001111');
  await expect(page).toHaveURL(/\/403$/);
  await expect(page.getByText('无权限')).toBeVisible();
});

test('管理员登录后可以查询校友档案', async ({ page }) => {
  await login(page, 'admin');
  await expect(page).toHaveURL(/\/admin\/alumni$/);
  await expect(page.getByRole('heading', { name: '校友管理' })).toBeVisible();

  await page.getByPlaceholder('姓名、单位、导师').fill('测试校友');
  await page.getByRole('button', { name: '查询' }).click();
  await expect(page.getByRole('row').filter({ hasText: '测试校友' })).toBeVisible();
});

test('管理员可以创建、编辑、刷新验证并清理校友档案', async ({ page }) => {
  const alumniName = `E2E校友${Date.now()}`;
  await login(page, 'admin');

  try {
    await page.getByRole('button', { name: '新增校友' }).click();
    const createDialog = page.getByRole('dialog', { name: '新增校友' });
    await createDialog.getByTestId('alumni-name-input').fill(alumniName);
    await createDialog.getByTestId('alumni-grade-input').fill('2026级');
    await createDialog.getByRole('combobox').first().click();
    await page.getByText('MPA专业学位研究生', { exact: true }).last().click();
    await createDialog.getByRole('button', { name: /确\s*定/ }).click();

    await page.getByPlaceholder('姓名、单位、导师').fill(alumniName);
    await page.getByRole('button', { name: '查询' }).click();
    const createdRow = page.getByRole('row').filter({ hasText: alumniName });
    await expect(createdRow).toBeVisible();
    await createdRow.getByRole('button', { name: '编辑' }).click();

    const editDialog = page.getByRole('dialog', { name: '编辑校友' });
    await editDialog.getByTestId('alumni-grade-input').fill('2027级');
    await editDialog.getByRole('button', { name: /确\s*定/ }).click();
    await expect(page.getByText('校友档案已更新')).toBeVisible();

    await page.reload();
    await expect(page.getByRole('row').filter({ hasText: alumniName })).toContainText('2027级');
  } finally {
    await removeAlumni(page, alumniName).catch(() => undefined);
  }
});

test('管理员可以上传、下载并删除校友附件', async ({ page }) => {
  const alumniName = `E2E附件校友${Date.now()}`;
  await login(page, 'admin');

  try {
    await page.getByRole('button', { name: '新增校友' }).click();
    const createDialog = page.getByRole('dialog', { name: '新增校友' });
    await createDialog.getByTestId('alumni-name-input').fill(alumniName);
    await createDialog.getByTestId('alumni-grade-input').fill('2026级');
    await createDialog.getByRole('combobox').first().click();
    await page.getByText('MPA专业学位研究生', { exact: true }).last().click();
    await createDialog.getByRole('button', { name: /确\s*定/ }).click();

    await page.getByRole('menuitem', { name: '校友服务' }).click();
    await page.getByPlaceholder('姓名、单位、导师').fill(alumniName);
    await page.getByRole('button', { name: '查询' }).click();
    const alumniRow = page.getByRole('row').filter({ hasText: alumniName });
    await alumniRow.getByRole('button', { name: '查看' }).click();
    await expect(page.getByRole('heading', { name: '校友详情' })).toBeVisible();

    await page
      .getByTestId('alumni-file-upload-degree_archive')
      .locator('input[type="file"]')
      .setInputFiles({
        name: attachmentName,
        mimeType: 'application/pdf',
        buffer: attachmentContent,
      });
    await expect(page.getByText('学位档案上传成功')).toBeVisible();
    const attachmentRow = page.getByRole('listitem').filter({ hasText: attachmentName });
    await expect(attachmentRow).toBeVisible();

    const downloadPromise = page.waitForEvent('download');
    await attachmentRow.getByRole('button', { name: '下载' }).click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toBe(attachmentName);
    const downloadPath = await download.path();
    expect(downloadPath).not.toBeNull();
    expect(await readFile(downloadPath!)).toEqual(attachmentContent);

    await attachmentRow.getByRole('button', { name: '删除' }).click();
    await page
      .getByRole('button', { name: /确\s*定/ })
      .last()
      .click();
    await expect(page.getByText('文件已删除')).toBeVisible();
    await expect(page.getByText(attachmentName)).toHaveCount(0);
  } finally {
    await removeAlumni(page, alumniName).catch(() => undefined);
  }
});
