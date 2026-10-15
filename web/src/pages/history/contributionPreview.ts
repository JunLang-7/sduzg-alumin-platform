import { historyApi } from '../../api/history';
import { ApiError } from '../../api/http';
import type { HistoryContribution } from '../../types/history';

export interface ContributionPreview {
  contribution: HistoryContribution;
  content: string;
  publishedVersion?: number;
}

// Keep the submitted snapshot intact; approved previews follow the live entry.
export async function loadContributionPreview(
  id: number,
  api: Pick<typeof historyApi, 'getContribution' | 'getEntry'> = historyApi,
): Promise<ContributionPreview> {
  const contribution = await api.getContribution(id);
  if (contribution.status === 'approved' && contribution.entry_id != null) {
    try {
      const entry = await api.getEntry(contribution.entry_id);
      return { contribution, content: entry.content, publishedVersion: entry.current_version };
    } catch (error) {
      // A deleted entry must not hide its author's original contribution.
      if (!(error instanceof ApiError) || error.status !== 404) throw error;
    }
  }
  return { contribution, content: contribution.content };
}
