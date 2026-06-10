import { request } from './http';
import type {
  ChangePasswordRequest,
  CurrentUser,
  LoginRequest,
  LoginResponse,
  SetupPasswordRequest,
  SetupPasswordResult,
  VerifyCodeRequest,
  VerifyCodeResult,
} from '../types/auth';

type MeResponse = CurrentUser | { user: CurrentUser };

function normalizeUser(payload: MeResponse): CurrentUser {
  return 'user' in payload ? payload.user : payload;
}

export const authApi = {
  async login(payload: LoginRequest) {
    return request<LoginResponse>({
      method: 'POST',
      url: '/auth/login',
      data: payload,
    });
  },

  async me() {
    const payload = await request<MeResponse>({
      method: 'GET',
      url: '/auth/me',
    });

    return normalizeUser(payload);
  },

  logout() {
    return request<void>({
      method: 'POST',
      url: '/auth/logout',
    });
  },

  changePassword(payload: ChangePasswordRequest) {
    return request<void>({
      method: 'POST',
      url: '/auth/change-password',
      data: payload,
    });
  },

  setupPassword(payload: SetupPasswordRequest) {
    return request<SetupPasswordResult>({
      method: 'POST',
      url: '/auth/setup-password',
      data: payload,
    });
  },

  sendVerifyCode(payload: VerifyCodeRequest) {
    return request<VerifyCodeResult>({
      method: 'POST',
      url: '/auth/verify-code/send',
      data: payload,
    });
  },
};
