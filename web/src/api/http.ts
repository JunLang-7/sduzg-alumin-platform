import axios, { AxiosError, type AxiosRequestConfig } from 'axios';
import type { ApiEnvelope } from '../types/common';

export class ApiError extends Error {
  code: number;
  status?: number;

  constructor(message: string, code: number, status?: number) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
  }
}

const client = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL ?? '/api/v1',
  timeout: 15000,
  withCredentials: true,
});

function isEnvelope<T>(value: unknown): value is ApiEnvelope<T> {
  return (
    typeof value === 'object' &&
    value !== null &&
    'code' in value &&
    'message' in value
  );
}

function redirectToLogin() {
  if (window.location.pathname !== '/login') {
    window.location.assign('/login');
  }
}

export async function request<T>(config: AxiosRequestConfig): Promise<T> {
  try {
    const response = await client.request<ApiEnvelope<T> | T>(config);
    const body = response.data;

    if (isEnvelope<T>(body)) {
      if (body.code !== 0) {
        if (body.code === 401) {
          redirectToLogin();
        }
        throw new ApiError(body.message || '请求失败', body.code, response.status);
      }

      return body.data as T;
    }

    return body as T;
  } catch (error) {
    if (error instanceof ApiError) {
      throw error;
    }

    const axiosError = error as AxiosError<ApiEnvelope<unknown>>;
    const status = axiosError.response?.status;
    const message =
      axiosError.response?.data?.message ||
      axiosError.message ||
      '网络异常，请稍后重试';

    if (status === 401) {
      redirectToLogin();
    }

    throw new ApiError(message, status ?? 500, status);
  }
}
