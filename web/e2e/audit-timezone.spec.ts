import { expect, test } from '@playwright/test';
import type { AuditOperation } from '../src/types/audit';

const record: AuditOperation = {
  id: 209,
  created_at: '2026-10-20T02:56:30Z',
  operator: '时区测试管理员',
  operator_role: 'super_admin',
  operator_role_label: '超级管理员',
  target_id: 203,
  target_type: 'alumni_profile',
  target_name: '时区测试校友',
  management_scope: 'MPA专业学位研究生',
  action: 'update',
  source: 'admin',
  status: 'applied',
};

for (const { zone, displayed, start, end } of [
  {
    zone: 'Asia/Shanghai',
    displayed: '2026-10-20 10:56',
    start: '2026-10-19T16:00:00.000Z',
    end: '2026-10-20T16:00:00.000Z',
  },
  {
    zone: 'America/New_York',
    displayed: '2026-10-19 22:56',
    start: '2026-10-20T04:00:00.000Z',
    end: '2026-10-21T04:00:00.000Z',
  },
]) {
  test.describe(`操作历史时区 ${zone}`, () => {
    test.use({ timezoneId: zone });

    test('列表、详情和校友历史卡片按同一时区展示，筛选发送本地日边界', async ({ page }) => {
      // 固定 API 时间，测试浏览器真实时区转换，而不是依赖测试机器或当前时间。
      await page.route('**/api/v1/admin/audit/operations**', async (route) => {
        const detail = new URL(route.request().url()).pathname.endsWith('/209');
        await route.fulfill({
          json: {
            code: 0,
            message: 'success',
            data: detail ? record : { items: [record], total: 1, page: 1, page_size: 20 },
          },
        });
      });
      await page.route('**/api/v1/alumni/203', (route) =>
        route.fulfill({
          json: { code: 0, data: { id: 203, name: record.target_name, grade: '2026级' } },
        }),
      );
      await page.route('**/api/v1/admin/alumni/203/files', (route) =>
        route.fulfill({ json: { code: 0, data: { items: [] } } }),
      );

      await page.goto('/login');
      await page.locator('input[autocomplete="username"]').fill('admin');
      await page.locator('input[autocomplete="current-password"]').fill('Admin@123456');
      await page.locator('button[type="submit"]').click();
      await page.waitForURL((url) => url.pathname !== '/login');
      await page.goto('/admin/audit/changes');

      const row = page.getByRole('row').filter({ hasText: record.operator });
      await expect(row).toContainText(displayed);
      await row.getByRole('button', { name: '详情' }).click();
      await expect(page.locator('.audit-drawer-heading p')).toHaveText(displayed);
      await page
        .getByRole('dialog', { name: '操作详情' })
        .getByRole('button', { name: '关闭', exact: true })
        .click();

      const inputs = page.locator('.audit-date-range input');
      await inputs.nth(0).fill('2026-10-20');
      await inputs.nth(0).press('Enter');
      const filteredRequest = page.waitForRequest((request) => {
        const url = new URL(request.url());
        return url.pathname === '/api/v1/admin/audit/operations' && url.searchParams.has('end_at');
      });
      await inputs.nth(1).fill('2026-10-20');
      await inputs.nth(1).press('Enter');
      const filter = new URL((await filteredRequest).url()).searchParams;
      expect(filter.get('start_at')).toBe(start);
      expect(filter.get('end_at')).toBe(end);
      expect(filter.has('start_date')).toBe(false);
      expect(filter.has('end_date')).toBe(false);

      await page.goto('/alumni/203');
      const history = page.locator('.audit-alumni-history-card');
      await expect(history.getByRole('row').filter({ hasText: record.operator })).toContainText(
        displayed,
      );
      await history.getByRole('button', { name: '详情' }).click();
      await expect(page.locator('.audit-drawer-heading p')).toHaveText(displayed);
    });
  });
}
