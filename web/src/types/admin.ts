import type { UserRole, UserStatus } from './auth';

export interface AdminUser {
  id: number;
  account: string;
  real_name?: string;
  mobile?: string;
  role: UserRole;
  status?: UserStatus;
  last_login_at?: string;
  created_at?: string;
}

export interface AdminQuery {
  page?: number;
  page_size?: number;
}

export interface CreateAdminPayload {
  account: string;
  password: string;
  real_name: string;
  mobile?: string;
}
