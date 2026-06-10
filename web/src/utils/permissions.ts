import type { CurrentUser, UserRole } from '../types/auth';

const roleWeight: Record<UserRole, number> = {
  alumni: 1,
  admin: 2,
  super_admin: 3,
};

const roleLabel: Record<UserRole, string> = {
  alumni: '校友',
  admin: '管理员',
  super_admin: '超级管理员',
};

export function getRoleLabel(role?: UserRole): string {
  if (!role) return '';
  return roleLabel[role] || role;
}

export function hasRole(user: CurrentUser | null, role: UserRole) {
  if (!user) {
    return false;
  }

  return roleWeight[user.role] >= roleWeight[role];
}

export function getDefaultPath(role?: UserRole) {
  if (role === 'admin' || role === 'super_admin') {
    return '/admin/alumni';
  }

  return '/alumni';
}
