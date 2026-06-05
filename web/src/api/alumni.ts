import { request } from './http';
import type { PageResult } from '../types/common';
import type {
  AlumniImportResult,
  AlumniFileItem,
  AlumniFileListResponse,
  AlumniProfile,
  AlumniProfilePayload,
  AlumniQuery,
  MyProfilePayload,
} from '../types/alumni';

export const alumniApi = {
  list(params: AlumniQuery) {
    return request<PageResult<AlumniProfile>>({
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

  importData(file: File) {
    const formData = new FormData();
    formData.append('file', file);

    return request<AlumniImportResult>({
      method: 'POST',
      url: '/admin/alumni/import',
      data: formData,
    });
  },
  
  listFiles(id: number) {
    return request<AlumniFileListResponse>({
      method: 'GET',
      url: `/admin/alumni/${id}/files`,
    });
  },

  uploadFile(id: number, fileType: string, file: File) {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('file_type', fileType);

    return request<AlumniFileItem>({
      method: 'POST',
      url: `/admin/alumni/${id}/files`,
      data: formData,
    });
  },

  deleteFile(alumniId: number, fileId: number) {
    return request<void>({
      method: 'DELETE',
      url: `/admin/alumni/${alumniId}/files/${fileId}`,
    });
  },
   
  downloadFile(alumniId: number, fileId: number) {
    return request<Blob>({
      method: 'GET',
      url: `/admin/alumni/${alumniId}/files/${fileId}/download`,
      responseType: 'blob',
    });
  },

  exportData(params: AlumniQuery & { format?: string }) {
    return request<Blob>({
      method: 'GET',
      url: '/admin/alumni/export',
      params,
      responseType: 'blob',
    });
  },
};
