import { create } from 'zustand';
import { authApi } from '../api/auth';
import type { ChangePasswordRequest, CurrentUser, LoginRequest } from '../types/auth';

const cacheKey = 'sdu_alumni_current_user';

function readCachedUser(): CurrentUser | null {
  try {
    const raw = window.localStorage.getItem(cacheKey);
    return raw ? (JSON.parse(raw) as CurrentUser) : null;
  } catch {
    return null;
  }
}

function cacheUser(user: CurrentUser | null) {
  if (user) {
    window.localStorage.setItem(cacheKey, JSON.stringify(user));
    return;
  }

  window.localStorage.removeItem(cacheKey);
}

interface AuthState {
  user: CurrentUser | null;
  sessionChecked: boolean;
  loading: boolean;
  login: (payload: LoginRequest) => Promise<CurrentUser>;
  ensureCurrentUser: () => Promise<CurrentUser | null>;
  logout: () => Promise<void>;
  changePassword: (payload: ChangePasswordRequest) => Promise<void>;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: readCachedUser(),
  sessionChecked: Boolean(readCachedUser()),
  loading: false,

  async login(payload) {
    set({ loading: true });
    try {
      const response = await authApi.login(payload);
      cacheUser(response.user);
      set({ user: response.user, sessionChecked: true, loading: false });
      return response.user;
    } catch (error) {
      set({ loading: false });
      throw error;
    }
  },

  async ensureCurrentUser() {
    const { user, sessionChecked } = get();
    if (user) {
      return user;
    }

    if (sessionChecked) {
      return null;
    }

    set({ loading: true });
    try {
      const currentUser = await authApi.me();
      cacheUser(currentUser);
      set({ user: currentUser, sessionChecked: true, loading: false });
      return currentUser;
    } catch {
      cacheUser(null);
      set({ user: null, sessionChecked: true, loading: false });
      return null;
    }
  },

  async logout() {
    try {
      await authApi.logout();
    } finally {
      cacheUser(null);
      set({ user: null, sessionChecked: true });
    }
  },

  changePassword(payload) {
    return authApi.changePassword(payload);
  },
}));
