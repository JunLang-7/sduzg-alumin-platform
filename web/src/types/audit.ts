import type { PageQuery } from './common';

export type AuditAction = 'create' | 'update' | 'delete' | 'import';

export type AuditStatus = 'applied' | 'conflicted';

export interface AuditChange {
  field_name: string;
  field_label: string;
  old_value: string;
  new_value: string;
  current_value?: string;
  sensitive?: boolean;
}

export interface AuditOperation {
  id: number;
  created_at: string;
  operator: string;
  operator_role: string;
  operator_role_label: string;
  target_id?: number;
  target_type: string;
  target_name: string;
  target_meta?: string;
  management_scope: string;
  action: AuditAction | string;
  source: string;
  reason?: string;
  status: AuditStatus | string;
  changes?: AuditChange[];
}

export interface AuditQuery extends PageQuery {
  start_date?: string;
  end_date?: string;
  management_scope?: string;
  action?: AuditAction;
  target_id?: number;
}

export const AUDIT_SCOPE_TAGS = [
  { value: '本科生', label: '本科生' },
  { value: '学术学位研究生', label: '学术学位研究生' },
  { value: 'MPA专业学位研究生', label: 'MPA专业学位研究生' },
] as const;

export const AUDIT_ACTION_OPTIONS: Array<{ value: AuditAction; label: string }> = [
  { value: 'create', label: '新增' },
  { value: 'update', label: '修改' },
  { value: 'delete', label: '删除' },
  { value: 'import', label: '导入' },
];
