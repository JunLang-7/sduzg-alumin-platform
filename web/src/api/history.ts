import { request } from './http';
import type {
  CreateHistoryContributionPayload,
  HistoryAttachmentUploadResult,
  HistoryContribution,
  HistoryAttachment,
  HistoryReviewAction,
  HistoryEntry,
  UploadHistoryAttachmentPayload,
  UpdateHistoryEntryPayload,
} from '../types/history';

const storageURL = (url: string) => {
  const target = new URL(url);
  return `${window.location.origin}${target.pathname}${target.search}`;
};

export const historyApi = {
  listEntries(keyword?: string) {
    return request<HistoryEntry[]>({
      method: 'GET',
      url: '/history/entries',
      params: keyword ? { keyword } : undefined,
    });
  },
  listMine() {
    return request<HistoryContribution[]>({ method: 'GET', url: '/history/contributions/me' });
  },
  getContribution(id: number) {
    return request<HistoryContribution>({ method: 'GET', url: `/history/contributions/${id}` });
  },
  createDraft(payload: CreateHistoryContributionPayload) {
    return request<HistoryContribution>({
      method: 'POST',
      url: '/history/contributions',
      data: payload,
    });
  },
  deleteDraft(id: number) {
    return request<{ deleted: boolean }>({
      method: 'DELETE',
      url: `/history/contributions/${id}`,
    });
  },
  updateContribution(id: number, payload: CreateHistoryContributionPayload) {
    return request<HistoryContribution>({
      method: 'PUT',
      url: `/history/contributions/${id}`,
      data: payload,
    });
  },
  updateEntry(id: number, payload: UpdateHistoryEntryPayload) {
    return request<HistoryEntry>({ method: 'PUT', url: `/history/entries/${id}`, data: payload });
  },
  submit(id: number) {
    return request<HistoryContribution>({
      method: 'POST',
      url: `/history/contributions/${id}/submit`,
    });
  },
  listPending() {
    return request<HistoryContribution[]>({ method: 'GET', url: '/history/reviews' });
  },
  listReviewAttachments(id: number) {
    return request<HistoryAttachment[]>({
      method: 'GET',
      url: `/history/reviews/${id}/attachments`,
    });
  },
  listContributionAttachments(id: number) {
    return request<HistoryAttachment[]>({
      method: 'GET',
      url: `/history/contributions/${id}/attachments`,
    });
  },
  deleteAttachment(contributionID: number, attachmentID: number) {
    return request<{ deleted: boolean }>({
      method: 'DELETE',
      url: `/history/contributions/${contributionID}/attachments/${attachmentID}`,
    });
  },
  updateAttachment(
    contributionID: number,
    attachmentID: number,
    payload: UploadHistoryAttachmentPayload,
  ) {
    return request<{ updated: boolean }>({
      method: 'PUT',
      url: `/history/contributions/${contributionID}/attachments/${attachmentID}`,
      data: payload,
    });
  },
  async previewAttachment(contributionID: number, attachmentID: number) {
    const result = await request<{ download_url: string }>({
      method: 'GET',
      url: `/history/contributions/${contributionID}/attachments/${attachmentID}/download`,
    });
    return storageURL(result.download_url);
  },
  review(id: number, action: HistoryReviewAction, review_comment = '') {
    return request<HistoryContribution>({
      method: 'POST',
      url: `/history/reviews/${id}`,
      data: { action, review_comment },
    });
  },
  async uploadAttachment(
    contributionID: number,
    file: File,
    payload: UploadHistoryAttachmentPayload,
  ) {
    const result = await request<HistoryAttachmentUploadResult>({
      method: 'POST',
      url: `/history/contributions/${contributionID}/attachments/upload-url`,
      data: { original_name: file.name, mime_type: file.type, ...payload },
    });
    const response = await fetch(storageURL(result.upload_url), {
      method: 'PUT',
      body: file,
      headers: { 'Content-Type': file.type },
    });
    if (!response.ok) throw new Error(`附件上传失败：${response.status}`);
    await request({
      method: 'POST',
      url: `/history/contributions/${contributionID}/attachments/${result.id}/confirm`,
    });
  },
};
