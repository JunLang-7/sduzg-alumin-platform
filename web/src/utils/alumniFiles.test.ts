import { describe, expect, it } from 'vitest';
import type { AlumniFileItem } from '../types/alumni';
import { draftFilesToActivate } from './alumniFiles';

const file = (id: number, status: AlumniFileItem['status']): AlumniFileItem => ({
  id,
  status,
  enrollment_id: 11,
  file_type: 'degree_archive',
  original_name: `${id}.pdf`,
  file_size: 10,
  mime_type: 'application/pdf',
  created_at: '',
});

describe('本次编辑的附件保存', () => {
  it('只激活当前编辑上传并保留的草稿', () => {
    expect(
      draftFilesToActivate([file(1, 'active'), file(2, 'draft'), file(3, 'draft')], [2, 4]),
    ).toEqual([2]);
  });
  it('仅打开并保存时不激活其他编辑窗口遗留的草稿', () => {
    expect(draftFilesToActivate([file(2, 'draft')], [])).toEqual([]);
  });
});
