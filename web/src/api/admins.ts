import { request } from './http';
import type { PageResult } from '../types/common';
import type { AdminQuery, AdminUser, CreateAdminPayload, UpdateAdminAccessPayload } from '../types/admin';
import type { DataDomain } from '../types/auth';

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

  detail(id: number) {
    return request<AdminUser>({ method: 'GET', url: `/super-admin/admins/${id}` });
  },

  replaceAccess(id: number, payload: UpdateAdminAccessPayload) {
    return request<AdminUser>({ method: 'PUT', url: `/super-admin/admins/${id}/access`, data: payload });
  },

  listDataDomains() {
    return request<DataDomain[]>({ method: 'GET', url: '/super-admin/data-domains' });
  },

  remove(id: number) {
    return request<void>({
      method: 'DELETE',
      url: `/super-admin/admins/${id}`,
    });
  },
};
