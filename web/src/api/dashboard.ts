import { request } from './http';
import type {
  DashboardDimension,
  DashboardOverview,
  DistributionItem,
} from '../types/dashboard';

export const dashboardApi = {
  overview() {
    return request<DashboardOverview>({
      method: 'GET',
      url: '/admin/dashboard/overview',
    });
  },

  distribution(dimension: DashboardDimension) {
    return request<DistributionItem[]>({
      method: 'GET',
      url: '/admin/dashboard/distribution',
      params: { dimension },
    });
  },
};
