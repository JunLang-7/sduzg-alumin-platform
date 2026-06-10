import { create } from 'zustand';
import { authApi } from '../api/auth';
import { cacheAccessToken, readAccessToken } from '../api/http';
import type { ChangePasswordRequest, CurrentUser, LoginRequest } from '../types/auth';

interface AuthState {
  user: CurrentUser | null;
  sessionChecked: boolean;
  loading: boolean;
  login: (payload: LoginRequest) => Promise<{ user: CurrentUser | null; registrationToken: string | null }>;
  ensureCurrentUser: () => Promise<CurrentUser | null>;
  logout: () => Promise<void>;
  changePassword: (payload: ChangePasswordRequest) => Promise<void>;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  sessionChecked: false,
  loading: false,

  async login(payload) {
    set({ loading: true });
    try {
      const response = await authApi.login(payload);
      if (response.registration_token) {
        set({ loading: false });
        return { user: null, registrationToken: response.registration_token };
      }
      cacheAccessToken(response.access_token);
      set({ user: response.user, sessionChecked: true, loading: false });
      return { user: response.user, registrationToken: null };
    } catch (error) {
      set({ loading: false });
      throw error;
    }
  },

  async ensureCurrentUser() {
    const { user, sessionChecked } = get();

    // 如果内存中已有用户信息，仅在 token 仍然存在时才直接返回
    if (user) {
      const token = readAccessToken();
      if (!token) {
        set({ user: null, sessionChecked: true });
        return null;
      }
      return user;
    }

    // 如果已经检查过且没有用户，返回 null
    if (sessionChecked) {
      return null;
    }

    // 检查是否有 token
    const token = readAccessToken();
    if (!token) {
      set({ sessionChecked: true });
      return null;
    }

    // 调用 API 获取用户信息
    set({ loading: true });
    try {
      const currentUser = await authApi.me();
      set({ user: currentUser, sessionChecked: true, loading: false });
      return currentUser;
    } catch {
      // 401/403 或其他错误，清除登录状态
      cacheAccessToken(null);
      set({ user: null, sessionChecked: true, loading: false });
      return null;
    }
  },

  async logout() {
    try {
      await authApi.logout();
    } finally {
      cacheAccessToken(null);
      set({ user: null, sessionChecked: true });
    }
  },

  changePassword(payload) {
    return authApi.changePassword(payload);
  },
}));
