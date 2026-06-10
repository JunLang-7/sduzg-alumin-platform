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
  email?: string;
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
  work_unit?: string;
  position?: string;
  mobile?: string;
}

export interface AlumniImportRowError {
  row: number;
  name: string;
  message: string;
}

export interface AlumniImportResult {
  total: number;
  success: number;
  errors: AlumniImportRowError[];
}

export type AlumniProfilePayload = Omit<AlumniProfile, 'id' | 'created_at' | 'updated_at'>;

export type MyProfilePayload = Pick<
  AlumniProfile,
  'work_unit' | 'position' | 'mailing_address' | 'mobile'
>;

export interface AlumniFileItem {
  id: number;
  file_type: 'degree_archive' | 'academic_record';
  original_name: string;
  file_size: number;
  mime_type: string;
  created_at: string;
}

export interface AlumniFileListResponse {
  alumni_id: number;
  degree_archive: AlumniFileItem[];
  academic_record: AlumniFileItem[];
}

export interface AlumniFileUploadURLResponse {
  file_id: number;
  upload_url: string;
  expires_in: number;
}

export interface AlumniFileDownloadURLResponse {
  download_url: string;
  expires_in: number;
  original_name: string;
}
