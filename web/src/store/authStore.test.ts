import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { authApi } from '../api/auth';
import type { CurrentUser } from '../types/auth';
import { useAuthStore } from './authStore';

vi.mock('../api/auth', () => ({
  authApi: { login: vi.fn(), me: vi.fn(), logout: vi.fn() },
}));

const tokenKey = 'sdu_alumni_access_token';
const user: CurrentUser = {
  id: 1,
  account: 'alumni',
  role: 'alumni',
  domains: [],
  permissions: [],
};
let values: Map<string, string>;
let storage: Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

beforeEach(() => {
  vi.resetAllMocks();
  values = new Map();
  storage = {
    getItem: vi.fn((key: string) => values.get(key) ?? null),
    setItem: vi.fn((key: string, value: string) => {
      values.set(key, value);
    }),
    removeItem: vi.fn((key: string) => {
      values.delete(key);
    }),
  };
  vi.stubGlobal('window', { localStorage: storage });
  useAuthStore.setState({ user: null, sessionChecked: false, loading: false });
  vi.mocked(authApi.login).mockResolvedValue({
    access_token: 'signed-token',
    token_type: 'Bearer',
    expires_at: '2099-01-01T00:00:00Z',
    user: { ...user, role: 'super_admin' },
  });
  vi.mocked(authApi.me).mockResolvedValue(user);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('database role is the source for client display', () => {
  it('uses auth/me after login and persists only the token', async () => {
    const result = await useAuthStore.getState().login({ account: 'alumni', password: 'test' });
    expect(result.user).toEqual(user);
    expect(useAuthStore.getState().user?.role).toBe('alumni');
    expect([...values]).toEqual([[tokenKey, 'signed-token']]);
    expect(storage.setItem).toHaveBeenCalledTimes(1);
  });

  it('ignores forged role and user records in localStorage when restoring a session', async () => {
    values.set(tokenKey, 'signed-token');
    values.set('role', 'super_admin');
    values.set('user', JSON.stringify({ ...user, role: 'super_admin' }));
    values.set(
      'auth-storage',
      JSON.stringify({ state: { user: { ...user, role: 'super_admin' } } }),
    );
    expect(await useAuthStore.getState().ensureCurrentUser()).toEqual(user);
    expect(storage.getItem).toHaveBeenCalledExactlyOnceWith(tokenKey);
    expect(authApi.me).toHaveBeenCalledOnce();
  });

  it('clears a new token and previous role if current-user verification fails', async () => {
    const failure = new Error('account disabled');
    useAuthStore.setState({ user: { ...user, role: 'super_admin' } });
    vi.mocked(authApi.me).mockRejectedValue(failure);
    await expect(
      useAuthStore.getState().login({ account: 'alumni', password: 'test' }),
    ).rejects.toThrow(failure);
    expect(values.has(tokenKey)).toBe(false);
    expect(useAuthStore.getState()).toMatchObject({
      user: null,
      sessionChecked: true,
      loading: false,
    });
  });

  it('clears a previous session when credentials are rejected', async () => {
    values.set(tokenKey, 'old-token');
    useAuthStore.setState({ user: { ...user, role: 'admin' }, sessionChecked: true });
    vi.mocked(authApi.login).mockRejectedValue(new Error('invalid credentials'));
    await expect(
      useAuthStore.getState().login({ account: 'alumni', password: 'wrong' }),
    ).rejects.toThrow('invalid credentials');
    expect(values.has(tokenKey)).toBe(false);
    expect(useAuthStore.getState().user).toBeNull();
    expect(authApi.me).not.toHaveBeenCalled();
  });

  it('does not turn a registration token into an authenticated session', async () => {
    values.set(tokenKey, 'old-token');
    useAuthStore.setState({ user: { ...user, role: 'admin' }, sessionChecked: true });
    vi.mocked(authApi.login).mockResolvedValue({
      access_token: '',
      token_type: 'Bearer',
      expires_at: '',
      user,
      registration_token: 'registration-only',
    });
    expect(await useAuthStore.getState().login({ mobile: '13800001111', code: '123456' })).toEqual({
      user: null,
      registrationToken: 'registration-only',
    });
    expect(values.has(tokenKey)).toBe(false);
    expect(useAuthStore.getState().user).toBeNull();
    expect(authApi.me).not.toHaveBeenCalled();
  });

  it('clears the session when auth/me rejects refresh', async () => {
    values.set(tokenKey, 'invalid-token');
    vi.mocked(authApi.me).mockRejectedValue(new Error('unauthorized'));
    expect(await useAuthStore.getState().ensureCurrentUser()).toBeNull();
    expect(values.has(tokenKey)).toBe(false);
    expect(useAuthStore.getState().user).toBeNull();
  });

  it('clears the role and token when logout fails', async () => {
    values.set(tokenKey, 'signed-token');
    useAuthStore.setState({ user });
    vi.mocked(authApi.logout).mockRejectedValue(new Error('offline'));
    await expect(useAuthStore.getState().logout()).rejects.toThrow('offline');
    expect(values.has(tokenKey)).toBe(false);
    expect(useAuthStore.getState().user).toBeNull();
  });
});
