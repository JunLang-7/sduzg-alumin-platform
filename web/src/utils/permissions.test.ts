import { describe, expect, it } from 'vitest';
import type { CurrentUser } from '../types/auth';
import { getDefaultPath, getRoleLabel, hasRole } from './permissions';

const alumni: CurrentUser = {
  id: 1,
  account: 'alumni',
  role: 'alumni',
  domains: [],
  permissions: [],
};

const admin: CurrentUser = { ...alumni, id: 2, role: 'admin' };
const superAdmin: CurrentUser = { ...alumni, id: 3, role: 'super_admin' };

describe('permissions', () => {
  it('enforces the role hierarchy and rejects anonymous users', () => {
    expect(hasRole(null, 'alumni')).toBe(false);
    expect(hasRole(alumni, 'admin')).toBe(false);
    expect(hasRole(admin, 'alumni')).toBe(true);
    expect(hasRole(superAdmin, 'admin')).toBe(true);
  });

  it('returns role labels and the appropriate landing path', () => {
    expect(getRoleLabel('super_admin')).toBe('超级管理员');
    expect(getDefaultPath('admin')).toBe('/admin/alumni');
    expect(getDefaultPath('alumni')).toBe('/profile');
  });
});
