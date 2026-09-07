import { AUDIT_ACTION_OPTIONS, type AuditAction, type AuditQuery } from '../types/audit';

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
