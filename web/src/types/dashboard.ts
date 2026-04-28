export interface DashboardOverview {
  total_alumni: number;
  total_accounts: number;
  mobile_complete_rate: number;
  work_unit_complete_rate: number;
  mentor_complete_rate: number;
}

export interface DistributionItem {
  name: string;
  value: number;
}

export type DashboardDimension =
  | 'grade'
  | 'class_name'
  | 'cohort'
  | 'gender'
  | 'major'
  | 'training_mode'
  | 'industry';
