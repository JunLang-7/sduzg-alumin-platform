import { describe, expect, it, vi } from 'vitest';
import type { HistoryContribution, HistoryEntry } from '../../types/history';
import { loadContributionPreview } from './contributionPreview';

const contribution: HistoryContribution = {
  id: 7,
  entry_id: 3,
  title: '学院概况',
  section_name: '',
  content: '原始投稿正文',
  source_note: '投稿资料',
  change_note: '',
  status: 'approved',
  updated_at: '2026-09-23T00:00:00Z',
};
const entry: HistoryEntry = {
  id: 3,
  title: '学院概况',
  summary: '',
  content: '管理员更新后的正式正文',
  source_note: '正式资料',
  current_version: 2,
  updated_at: '2026-09-29T00:00:00Z',
};

describe('contribution preview', () => {
  it('previews the latest published body without overwriting the submitted snapshot', async () => {
    const api = {
      getContribution: vi.fn().mockResolvedValue(contribution),
      getEntry: vi.fn().mockResolvedValue(entry),
    };
    const result = await loadContributionPreview(7, api);
    expect(api.getContribution).toHaveBeenCalledWith(7);
    expect(api.getEntry).toHaveBeenCalledWith(3);
    expect(result.content).toBe(entry.content);
    expect(result.publishedVersion).toBe(2);
    expect(result.contribution.content).toBe('原始投稿正文');
    api.getEntry.mockResolvedValue({ ...entry, content: '再次修改', current_version: 3 });
    expect((await loadContributionPreview(7, api)).content).toBe('再次修改');
  });

  it.each(['draft', 'pending', 'returned', 'rejected'] as const)(
    'reads freshly saved %s content rather than the published entry',
    async (status) => {
      const current = { ...contribution, status, content: '最新保存的投稿正文' };
      const api = {
        getContribution: vi.fn().mockResolvedValue(current),
        getEntry: vi.fn(),
      };
      expect(await loadContributionPreview(7, api)).toEqual({
        contribution: current,
        content: current.content,
      });
      expect(api.getEntry).not.toHaveBeenCalled();
    },
  );

  it('keeps the submitted body when an approved contribution has no linked entry', async () => {
    const current = { ...contribution, entry_id: undefined };
    const api = { getContribution: vi.fn().mockResolvedValue(current), getEntry: vi.fn() };
    expect((await loadContributionPreview(7, api)).content).toBe(current.content);
    expect(api.getEntry).not.toHaveBeenCalled();
  });

  it('surfaces published-entry failures instead of silently showing stale content', async () => {
    const api = {
      getContribution: vi.fn().mockResolvedValue(contribution),
      getEntry: vi.fn().mockRejectedValue(new Error('正文加载失败')),
    };
    await expect(loadContributionPreview(7, api)).rejects.toThrow('正文加载失败');
  });

  it('does not fetch a published entry when the private contribution request is denied', async () => {
    const api = {
      getContribution: vi.fn().mockRejectedValue(new Error('权限不足')),
      getEntry: vi.fn(),
    };
    await expect(loadContributionPreview(7, api)).rejects.toThrow('权限不足');
    expect(api.getEntry).not.toHaveBeenCalled();
  });
});
