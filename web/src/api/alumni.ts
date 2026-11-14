import { request } from './http';
import type { PageResult } from '../types/common';
import type {
  AlumniImportResult,
  AlumniFileItem,
  AlumniFileListResponse,
  AlumniFileType,
  AlumniFileUploadURLResponse,
  AlumniFileDownloadURLResponse,
  AlumniListItem,
  AlumniProfile,
  AlumniProfilePayload,
  AlumniQuery,
  MyProfilePayload,
} from '../types/alumni';

const toSameOriginStorageURL = (presignedURL: string) => {
  const url = new URL(presignedURL, window.location.origin);
  return `${url.pathname}${url.search}`;
};

export const alumniApi = {
  list(params: AlumniQuery) {
    return request<PageResult<AlumniListItem>>({
      method: 'GET',
      url: '/alumni',
      params,
    });
  },

  detail(id: number | string) {
    return request<AlumniProfile>({
      method: 'GET',
      url: `/alumni/${id}`,
    });
  },

  myProfile() {
    return request<AlumniProfile>({
      method: 'GET',
      url: '/alumni/me',
    });
  },

  updateMyProfile(payload: MyProfilePayload) {
    return request<AlumniProfile>({
      method: 'PUT',
      url: '/alumni/me',
      data: payload,
    });
  },

  updateContact(payload: { mobile?: string; email?: string; code: string }) {
    return request<void>({
      method: 'PUT',
      url: '/alumni/me/contact',
      data: payload,
    });
  },

  create(payload: AlumniProfilePayload) {
    return request<AlumniProfile>({
      method: 'POST',
      url: '/admin/alumni',
      data: payload,
    });
  },

  update(id: number, payload: AlumniProfilePayload) {
    return request<AlumniProfile>({
      method: 'PUT',
      url: `/admin/alumni/${id}`,
      data: payload,
    });
  },

  remove(id: number) {
    return request<void>({
      method: 'DELETE',
      url: `/admin/alumni/${id}`,
    });
  },

  importData(file: File, dataDomainID?: number) {
    const formData = new FormData();
    formData.append('file', file);
    if (dataDomainID) formData.append('data_domain_id', String(dataDomainID));

    return request<AlumniImportResult>({
      method: 'POST',
      url: '/admin/alumni/import',
      data: formData,
    });
  },

  listFiles(id: number, includeDraft = false) {
    return request<AlumniFileListResponse>({
      method: 'GET',
      url: `/admin/alumni/${id}/files`,
      params: includeDraft ? { include_draft: true } : undefined,
    });
  },

  async uploadFile(
    id: number,
    enrollmentId: number,
    fileType: AlumniFileType,
    file: File,
  ): Promise<AlumniFileItem> {
    // 1. 请求预签名上传 URL（绑定到具体学籍段）
    const { file_id, upload_url } = await request<AlumniFileUploadURLResponse>({
      method: 'POST',
      url: `/admin/alumni/${id}/enrollments/${enrollmentId}/files/upload-url`,
      data: {
        file_type: fileType,
        original_name: file.name,
        mime_type: file.type || 'application/octet-stream',
      },
    });

    // 2. 通过当前站点的 Nginx 代理直传 MinIO，避免公开地址和 CORS 配置不一致
    const putResp = await fetch(toSameOriginStorageURL(upload_url), {
      method: 'PUT',
      body: file,
      headers: { 'Content-Type': file.type || 'application/octet-stream' },
    });
    if (!putResp.ok) {
      throw new Error(`文件上传失败: ${putResp.status}`);
    }

    // 3. 确认直传完成；文件仍是隔离草稿，保存全部修改后才转正式附件。
    await request({
      method: 'POST',
      url: `/admin/alumni/${id}/enrollments/${enrollmentId}/files/${file_id}/confirm`,
    });

    return {
      id: file_id,
      enrollment_id: enrollmentId,
      file_type: fileType,
      original_name: file.name,
      file_size: file.size,
      mime_type: file.type || 'application/octet-stream',
      status: 'draft',
      created_at: new Date().toISOString(),
    };
  },

  deleteDraftFile(alumniId: number, enrollmentId: number, fileId: number) {
    return request<void>({
      method: 'DELETE',
      url: `/admin/alumni/${alumniId}/enrollments/${enrollmentId}/files/${fileId}`,
    });
  },

  async downloadFile(alumniId: number, enrollmentId: number, fileId: number) {
    const { download_url, original_name } = await request<AlumniFileDownloadURLResponse>({
      method: 'GET',
      url: `/admin/alumni/${alumniId}/enrollments/${enrollmentId}/files/${fileId}/download`,
    });
    // 通过 fetch 获取文件再创建 blob URL 下载，绕过弹窗拦截且保证下载行为
    const fileResp = await fetch(toSameOriginStorageURL(download_url));
    if (!fileResp.ok) {
      throw new Error(`下载失败: ${fileResp.status}`);
    }
    const blob = await fileResp.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = original_name;
    a.click();
    window.URL.revokeObjectURL(url);
  },

  async getDownloadURL(alumniId: number, enrollmentId: number, fileId: number) {
    const { download_url } = await request<AlumniFileDownloadURLResponse>({
      method: 'GET',
      url: `/admin/alumni/${alumniId}/enrollments/${enrollmentId}/files/${fileId}/download`,
    });
    return toSameOriginStorageURL(download_url);
  },

  exportData(params: AlumniQuery & { format?: string }) {
    return request<Blob>({
      method: 'GET',
      url: '/admin/alumni/export',
      params,
      responseType: 'blob',
    });
  },

  downloadTemplate() {
    return request<Blob>({
      method: 'GET',
      url: '/admin/alumni/template',
      responseType: 'blob',
    });
  },
};
