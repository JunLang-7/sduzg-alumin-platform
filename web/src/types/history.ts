export type HistoryContributionStatus = 'draft' | 'pending' | 'returned' | 'approved' | 'rejected';

export interface HistoryEntry {
  can_delete?: boolean;
  can_request_deletion?: boolean;
  id: number;
  title: string;
  summary: string;
  content: string;
  source_note: string;
  current_version: number;
  updated_at: string;
}

export interface HistoryContribution {
  id: number;
  entry_id?: number;
  title: string;
  section_name: string;
  content: string;
  source_note: string;
  change_note: string;
  status: HistoryContributionStatus;
  data_domain_id?: number;
  review_comment?: string;
  submitted_at?: string;
  updated_at: string;
}

export interface HistoryAttachment {
  id: number;
  original_name: string;
  mime_type: string;
  file_size: number;
  description: string;
  source_note: string;
  rights_note: string;
  consent_confirmed: boolean;
}

export type HistoryReviewAction = 'approve' | 'return' | 'reject';

export interface HistoryDeletionRequest {
  id: number;
  entry_id: number;
  title: string;
  requested_by: number;
  reason: string;
  status: 'pending' | 'approved' | 'rejected';
  review_comment: string;
  reviewed_at?: string;
  created_at: string;
}

export interface CreateHistoryContributionPayload {
  entry_id?: number;
  data_domain_id?: number;
  title: string;
  section_name?: string;
  content: string;
  source_note: string;
  change_note?: string;
}

export interface UpdateHistoryEntryPayload {
  title: string;
  content: string;
  source_note: string;
  change_note?: string;
}

export interface UploadHistoryAttachmentPayload {
  description: string;
  source_note: string;
  rights_note: string;
  consent_confirmed: boolean;
}

interface HistoryAttachmentUploadResult {
  id: number;
  upload_url: string;
}

export type { HistoryAttachmentUploadResult };
