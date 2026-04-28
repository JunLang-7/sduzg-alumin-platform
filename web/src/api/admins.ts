import { request } from './http';
import type { PageResult } from '../types/common';
import type { AdminQuery, AdminUser, CreateAdminPayload } from '../types/admin';

export const adminsApi = {
  list(params: AdminQuery) {
    return request<PageResult<AdminUser>>({
      method: 'GET',
      url: '/super-admin/admins',
      params,
    });
  },

  create(payload: CreateAdminPayload) {
    return request<AdminUser>({
      method: 'POST',
      url: '/super-admin/admins',
      data: payload,
    });
  },

  remove(id: number) {
    return request<void>({
      method: 'DELETE',
      url: `/super-admin/admins/${id}`,
    });
  },
};
