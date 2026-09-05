import type { CurrentUser } from '../types/auth';

// 当前项目未配置前端单元测试框架；权限判断集中为无副作用函数，由 TypeScript 构建校验，后续可直接接入单测覆盖。
export const permissionCodes = {
  sensitiveRead: 'alumni.sensitive.read',
  filesManage: 'alumni.files.manage',
} as const;

export function hasCapability(user: CurrentUser | null | undefined, permission: string): boolean {
  return user?.role === 'super_admin' || Boolean(user?.permissions?.includes(permission));
}

export function canReadSensitive(user: CurrentUser | null | undefined): boolean {
  return hasCapability(user, permissionCodes.sensitiveRead);
}

export function canManageFiles(user: CurrentUser | null | undefined): boolean {
  return hasCapability(user, permissionCodes.filesManage);
}
