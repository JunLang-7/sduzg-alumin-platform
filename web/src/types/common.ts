export interface ApiEnvelope<T> {
  code: number;
  message: string;
  data?: T;
}

export interface PageResult<T> {
  items: T[];
  page: number;
  page_size: number;
  total: number;
}

export interface PageQuery {
  page?: number;
  page_size?: number;
}
