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
  account?: string;
  mobile?: string;
  email?: string;
  password?: string;
  code?: string;
  grant_type?: 'password' | 'sms_code' | 'email_code';
}

export interface LoginResponse {
  access_token: string;
  token_type: 'Bearer';
  expires_at: string;
  user: CurrentUser;
  registration_token?: string;
}

export interface ChangePasswordRequest {
  old_password: string;
  new_password: string;
  confirm_password: string;
}

export interface VerifyCodeRequest {
  target: string;
  purpose: 'login';
}

export interface SetupPasswordRequest {
  registration_token: string;
  new_password: string;
  confirm_password: string;
}

export interface SetupPasswordResult {
  access_token: string;
  token_type: 'Bearer';
  expires_at: string;
  user: CurrentUser;
}

export interface VerifyCodeResult {
  expire_at: string;
  resend_after: number;
}
