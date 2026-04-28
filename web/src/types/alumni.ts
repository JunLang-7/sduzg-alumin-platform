export type AlumniStatus = 'active' | 'deleted';

export interface AlumniProfile {
  id: number;
  name: string;
  grade: string;
  class_name?: string;
  cohort?: string;
  counselor?: string;
  mentor?: string;
  major?: string;
  training_mode?: string;
  industry?: string;
  work_unit?: string;
  position?: string;
  mailing_address?: string;
  gender?: string;
  mobile?: string;
  remark?: string;
  status?: AlumniStatus;
  created_at?: string;
  updated_at?: string;
}

export interface AlumniQuery {
  page?: number;
  page_size?: number;
  keyword?: string;
  grade?: string;
  class_name?: string;
  cohort?: string;
  major?: string;
  training_mode?: string;
  industry?: string;
}

export type AlumniProfilePayload = Omit<AlumniProfile, 'id' | 'created_at' | 'updated_at'>;

export type MyProfilePayload = Pick<
  AlumniProfile,
  'work_unit' | 'position' | 'mailing_address' | 'mobile'
>;
