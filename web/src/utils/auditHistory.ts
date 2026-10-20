import {
  AUDIT_ACTION_OPTIONS,
  type AuditAction,
  type AuditBatchField,
  type AuditQuery,
} from '../types/audit';
import { formatDateTime } from './datetime';

const ACTION_LABELS: Record<string, string> = Object.fromEntries(
  AUDIT_ACTION_OPTIONS.map((item) => [item.value, item.label]),
);

const ACTION_COLORS: Record<string, string> = {
  create: 'green',
  update: 'blue',
  delete: 'red',
  import: 'gold',
};

export interface AuditQueryInput {
  page: number;
  pageSize: number;
  startDate?: string;
  endDate?: string;
  managementScope?: string;
  action?: AuditAction;
  targetId?: number;
}

export function getAuditActionLabel(action: string): string {
  return ACTION_LABELS[action] || action || '操作';
}

export function getAuditActionColor(action: string): string | undefined {
  return ACTION_COLORS[action];
}

export function formatAuditDateTime(value: string): string {
  return formatDateTime(value);
}

// 日期筛选按浏览器本地日边界换算为 UTC 瞬时：开始日 00:00，结束日次日 00:00（右开）。
function localDayBoundaryISO(date: string, nextDay: boolean): string | undefined {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date);
  if (!match) return undefined;
  const value = new Date(
    Number(match[1]),
    Number(match[2]) - 1,
    Number(match[3]) + (nextDay ? 1 : 0),
    0,
    0,
    0,
    0,
  );
  return Number.isNaN(value.getTime()) ? undefined : value.toISOString();
}

export function buildAuditQuery(input: AuditQueryInput): AuditQuery {
  const startAt = input.startDate ? localDayBoundaryISO(input.startDate, false) : undefined;
  const endAt = input.endDate ? localDayBoundaryISO(input.endDate, true) : undefined;
  return {
    page: input.page,
    page_size: input.pageSize,
    ...(startAt ? { start_at: startAt } : {}),
    ...(endAt ? { end_at: endAt } : {}),
    ...(input.managementScope ? { management_scope: input.managementScope } : {}),
    ...(input.action ? { action: input.action } : {}),
    ...(input.targetId !== undefined ? { target_id: input.targetId } : {}),
  };
}

export function buildAlumniHistoryQuery(alumniId: number, page = 1, pageSize = 20): AuditQuery {
  return buildAuditQuery({ page, pageSize, targetId: alumniId });
}

// 批量导入审计只保存有限条明细；旧记录缺少 total 字段时按实际条数兜底。
export function getAuditBatchRecordSummary(record: {
  batch_created_alumni?: unknown[];
  batch_created_alumni_total?: number;
  batch_created_alumni_truncated?: boolean;
}): { displayed: number; total: number; truncated: boolean } {
  const displayed = record.batch_created_alumni?.length ?? 0;
  const total = Math.max(record.batch_created_alumni_total || 0, displayed);
  return {
    displayed,
    total,
    truncated: Boolean(record.batch_created_alumni_truncated) || total > displayed,
  };
}

export const AUDIT_BATCH_PUBLIC_FIELDS: AuditBatchField[] = [
  { field_name: 'name', field_label: '姓名' },
  { field_name: 'grade', field_label: '年级' },
  { field_name: 'class_name', field_label: '班级' },
  { field_name: 'major', field_label: '专业' },
  { field_name: 'training_mode', field_label: '培养方式' },
];

export const AUDIT_BATCH_SENSITIVE_FIELDS: AuditBatchField[] = [
  { field_name: 'mobile', field_label: '手机号', sensitive: true },
  { field_name: 'email', field_label: '邮箱', sensitive: true },
  { field_name: 'work_unit', field_label: '工作单位', sensitive: true },
  { field_name: 'position', field_label: '职务', sensitive: true },
  { field_name: 'mailing_address', field_label: '通讯地址', sensitive: true },
];

// 批量导入明细只允许展示固定字段，敏感字段由当前登录用户的权限决定。
export function getAuditBatchVisibleFields(
  fields: AuditBatchField[] | undefined,
  sensitiveReadable: boolean,
): AuditBatchField[] {
  const fieldMap = new Map((fields || []).map((field) => [field.field_name, field]));
  const publicFields = AUDIT_BATCH_PUBLIC_FIELDS.map(
    (field) => fieldMap.get(field.field_name) || field,
  );
  const sensitiveFields = AUDIT_BATCH_SENSITIVE_FIELDS.map(
    (field) => fieldMap.get(field.field_name) || field,
  );

  return sensitiveReadable ? [...publicFields, ...sensitiveFields] : publicFields;
}
