import type { AlumniFileItem } from '../types/alumni';

// 保存只激活本次编辑上传且仍保留的草稿，其他窗口或先前编辑的草稿不参与。
export function draftFilesToActivate(
  files: AlumniFileItem[],
  uploadedDraftIds: number[],
): number[] {
  const uploaded = new Set(uploadedDraftIds);
  return files
    .filter((file) => file.status === 'draft' && uploaded.has(file.id))
    .map((file) => file.id);
}
