import {
  AUDIT_ACTION_OPTIONS,
  type AuditAction,
  type AuditBatchField,
  type AuditQuery,
} from '../types/audit';

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
  return value.replace('T', ' ').slice(0, 16);
}

export function buildAuditQuery(input: AuditQueryInput): AuditQuery {
  return {
    page: input.page,
    page_size: input.pageSize,
    ...(input.startDate ? { start_date: input.startDate } : {}),
    ...(input.endDate ? { end_date: input.endDate } : {}),
    ...(input.managementScope ? { management_scope: input.managementScope } : {}),
    ...(input.action ? { action: input.action } : {}),
    ...(input.targetId !== undefined ? { target_id: input.targetId } : {}),
  };
}

export function buildAlumniHistoryQuery(alumniId: number, page = 1, pageSize = 20): AuditQuery {
  return buildAuditQuery({ page, pageSize, targetId: alumniId });
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
