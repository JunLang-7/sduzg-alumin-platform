export type UserRole = 'alumni' | 'admin' | 'super_admin';

export type UserStatus = 'active' | 'disabled' | 'deleted';

export interface CurrentUser {
  id: number;
  account: string;
  role: UserRole;
  real_name?: string;
  mobile?: string;
  status?: UserStatus;
  alumni_id?: number;
}

export interface LoginRequest {
  account: string;
  password: string;
}

export interface LoginResponse {
  user: CurrentUser;
}

export interface ChangePasswordRequest {
  old_password: string;
  new_password: string;
  confirm_password: string;
}
