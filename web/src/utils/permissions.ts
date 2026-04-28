import type { CurrentUser, UserRole } from '../types/auth';

const roleWeight: Record<UserRole, number> = {
  alumni: 1,
  admin: 2,
  super_admin: 3,
};

export function hasRole(user: CurrentUser | null, role: UserRole) {
  if (!user) {
    return false;
  }

  return roleWeight[user.role] >= roleWeight[role];
}

export function getDefaultPath(role?: UserRole) {
  if (role === 'admin' || role === 'super_admin') {
    return '/admin/dashboard';
  }

  return '/alumni';
}
