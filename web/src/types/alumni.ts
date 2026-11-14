export type AlumniStatus = 'active' | 'deleted';

export type TrainingLevel = 'undergraduate' | 'master' | 'doctor' | 'unverified';

export const TRAINING_LEVEL_LABELS: Record<TrainingLevel, string> = {
  undergraduate: '本科',
  master: '硕士',
  doctor: '博士',
  unverified: '待核验',
};

/** 新增学籍时只允许明确选择三种层次，待核验仅用于历史数据。 */
export const EXPLICIT_TRAINING_LEVELS: TrainingLevel[] = ['undergraduate', 'master', 'doctor'];

export interface AlumniEnrollment {
  id: number;
  data_domain_id: number;
  data_domain_name?: string;
  training_level: TrainingLevel;
  grade: string;
  class_name?: string;
  cohort?: string;
  counselor?: string;
  mentor?: string;
  major?: string;
  training_mode?: string;
  remark?: string;
  version: number;
  created_at?: string;
  updated_at?: string;
}

export interface AlumniEnrollmentInput {
  id?: number;
  version?: number;
  data_domain_id: number;
  training_level: TrainingLevel;
  grade: string;
  class_name?: string;
  cohort?: string;
  counselor?: string;
  mentor?: string;
  major?: string;
  training_mode?: string;
  remark?: string;
  activate_file_ids?: number[];
  remove_file_ids?: number[];
}

export interface AlumniEnrollmentSummary {
  id: number;
  data_domain_id: number;
  data_domain_name?: string;
  training_level: TrainingLevel;
  grade: string;
  class_name?: string;
  cohort?: string;
  counselor?: string;
  mentor?: string;
  major?: string;
  training_mode?: string;
}

export interface AlumniProfile {
  id: number;
  name: string;
  gender?: string;
  mobile?: string;
  email?: string;
  industry?: string;
  work_unit?: string;
  position?: string;
  mailing_address?: string;
  status?: AlumniStatus;
  version: number;
  created_at?: string;
  updated_at?: string;
  enrollments: AlumniEnrollment[];
}

export interface AlumniListItem {
  id: number;
  name: string;
  gender?: string;
  industry?: string;
  work_unit?: string;
  position?: string;
  mobile?: string;
  email?: string;
  version: number;
  updated_at?: string;
  enrollments: AlumniEnrollmentSummary[];
}

export interface AlumniQuery {
  page?: number;
  page_size?: number;
  keyword?: string;
  grade?: string;
  class_name?: string;
  cohort?: string;
  counselor?: string;
  mentor?: string;
  major?: string;
  training_mode?: string;
  training_level?: TrainingLevel;
  industry?: string;
  work_unit?: string;
  position?: string;
  mobile?: string;
  data_domain_id?: number;
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

/** 新增/编辑提交：个人信息共用一份，学籍逐段提交；版本号用于并发冲突检测。 */
export interface AlumniProfilePayload {
  version?: number;
  name: string;
  gender?: string;
  mobile?: string;
  email?: string;
  industry?: string;
  work_unit?: string;
  position?: string;
  mailing_address?: string;
  enrollments: AlumniEnrollmentInput[];
}

export type MyProfilePayload = Pick<
  AlumniProfile,
  'work_unit' | 'position' | 'mailing_address' | 'mobile'
>;

export type AlumniFileType = 'degree_archive' | 'academic_record';

export interface AlumniFileItem {
  id: number;
  enrollment_id: number;
  file_type: AlumniFileType;
  original_name: string;
  file_size: number;
  mime_type: string;
  status: 'active' | 'draft' | 'pending';
  created_at: string;
}

export interface AlumniEnrollmentFileGroup {
  enrollment_id: number;
  degree_archive: AlumniFileItem[];
  academic_record: AlumniFileItem[];
}

export interface AlumniFileListResponse {
  alumni_id: number;
  enrollments: AlumniEnrollmentFileGroup[];
}

export interface AlumniFileUploadURLResponse {
  file_id: number;
  enrollment_id: number;
  upload_url: string;
  expires_in: number;
}

export interface AlumniFileDownloadURLResponse {
  download_url: string;
  expires_in: number;
  original_name: string;
}
