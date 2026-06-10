import { useEffect, useState } from 'react';
import {
  DeleteOutlined,
  DownloadOutlined,
  EyeOutlined,
  FileTextOutlined,
  IdcardOutlined,
  LoadingOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import {
  Button,
  Descriptions,
  Empty,
  Modal,
  Popconfirm,
  Spin,
  Upload,
  message,
} from 'antd';
import { alumniApi } from '../../api/alumni';
import type {
  AlumniFileItem,
  AlumniFileListResponse,
  AlumniProfile,
} from '../../types/alumni';

interface AlumniDetailModalProps {
  profile: AlumniProfile | null;
  loading: boolean;
  open: boolean;
  onClose: () => void;
}

type ArchiveType = 'academic_record' | 'degree_archive';

function displayValue(value?: string) {
  return value?.trim() || '未填';
}

function displayTime(value?: string) {
  if (!value) return '未填';
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? value
    : new Intl.DateTimeFormat('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
      }).format(date);
}

function formatFileSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function getErrorMessage(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? error.message : fallback;
}

function getMimeType(item: AlumniFileItem) {
  return (item.mime_type || '').toLowerCase();
}

export function AlumniDetailModal({
  profile,
  loading,
  open,
  onClose,
}: AlumniDetailModalProps) {
  const [files, setFiles] = useState<AlumniFileListResponse | null>(null);
  const [filesLoading, setFilesLoading] = useState(false);
  const [activeArchive, setActiveArchive] = useState<ArchiveType | null>(null);
  const [uploadingArchive, setUploadingArchive] = useState<ArchiveType | null>(null);
  const [previewItem, setPreviewItem] = useState<AlumniFileItem | null>(null);
  const [previewUrl, setPreviewUrl] = useState('');
  const [previewLoading, setPreviewLoading] = useState(false);
  const [deletingFileId, setDeletingFileId] = useState<number | null>(null);

  const loadFiles = async (alumniId: number) => {
    setFiles(await alumniApi.listFiles(alumniId));
  };

  const closePreview = () => {
    setPreviewUrl('');
    setPreviewItem(null);
  };

  useEffect(() => {
    if (!open || !profile?.id) {
      setFiles(null);
      setActiveArchive(null);
      closePreview();
      return;
    }

    let active = true;
    setActiveArchive(null);
    setFilesLoading(true);
    alumniApi
      .listFiles(profile.id)
      .then((result) => {
        if (active) setFiles(result);
      })
      .catch((error: unknown) => {
        if (active) {
          setFiles({
            alumni_id: profile.id,
            degree_archive: [],
            academic_record: [],
          });
          message.warning(getErrorMessage(error, '档案接口不可用，请检查文件存储服务'));
        }
      })
      .finally(() => {
        if (active) setFilesLoading(false);
      });

    return () => {
      active = false;
    };
  }, [open, profile?.id]);

  const downloadFile = async (item: AlumniFileItem) => {
    if (!profile) return;
    try {
      await alumniApi.downloadFile(profile.id, item.id);
    } catch (error) {
      message.error(getErrorMessage(error, '档案下载失败'));
    }
  };

  const previewFile = async (item: AlumniFileItem) => {
    if (!profile) return;
    const mimeType = getMimeType(item);
    if (!mimeType.startsWith('image/') && mimeType !== 'application/pdf') {
      message.info('Word 文件暂不支持浏览器内预览，已为你下载');
      await downloadFile(item);
      return;
    }

    closePreview();
    setPreviewItem(item);
    setPreviewLoading(true);
    try {
      const downloadUrl = await alumniApi.getDownloadURL(profile.id, item.id);
      setPreviewUrl(downloadUrl);
    } catch (error) {
      setPreviewItem(null);
      message.error(getErrorMessage(error, '档案预览加载失败'));
    } finally {
      setPreviewLoading(false);
    }
  };

  const uploadFile = async (fileType: ArchiveType, file: File) => {
    if (!profile) return;
    setUploadingArchive(fileType);
    try {
      await alumniApi.uploadFile(profile.id, fileType, file);
      message.success(`${fileType === 'academic_record' ? '学籍档案' : '学位档案'}上传成功`);
    } catch (error) {
      message.error(
        getErrorMessage(error, '上传失败，请确认文件存储可用且文件不超过 50MB'),
      );
      setUploadingArchive(null);
      return;
    }

    try {
      await loadFiles(profile.id);
    } catch (error) {
      message.warning(
        `档案上传成功，但列表刷新失败：${getErrorMessage(error, '请稍后重试')}`,
      );
    } finally {
      setUploadingArchive(null);
    }
  };

  const deleteFile = async (item: AlumniFileItem) => {
    if (!profile) return;
    setDeletingFileId(item.id);
    try {
      await alumniApi.deleteFile(profile.id, item.id);
      if (previewItem?.id === item.id) {
        closePreview();
      }
      message.success('档案已删除');
    } catch (error) {
      message.error(getErrorMessage(error, '档案删除失败'));
      setDeletingFileId(null);
      return;
    }

    try {
      await loadFiles(profile.id);
    } catch (error) {
      message.warning(
        `档案已删除，但列表刷新失败：${getErrorMessage(error, '请稍后重试')}`,
      );
    } finally {
      setDeletingFileId(null);
    }
  };

  const renderFiles = (
    title: string,
    items: AlumniFileItem[],
    fileType: ArchiveType,
  ) => (
    <section className="dashboard-archive-section">
      <header>
        <span><FileTextOutlined /></span>
        <strong>{title}</strong>
        <em>{items.length} 个文件</em>
        <Upload
          accept=".pdf,.doc,.docx,.jpg,.jpeg,.png"
          showUploadList={false}
          beforeUpload={(file) => {
            void uploadFile(fileType, file as File);
            return false;
          }}
          disabled={uploadingArchive !== null}
        >
          <Button
            type="text"
            size="small"
            icon={uploadingArchive === fileType ? <LoadingOutlined /> : <UploadOutlined />}
            disabled={uploadingArchive !== null}
          >
            {uploadingArchive === fileType ? '上传中' : '上传'}
          </Button>
        </Upload>
      </header>
      {items.length ? (
        <div className="dashboard-archive-list">
          {items.map((item) => (
            <div
              key={item.id}
              role="button"
              tabIndex={0}
              aria-label={`查看${item.original_name}`}
              onClick={() => void previewFile(item)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  void previewFile(item);
                }
              }}
            >
              <FileTextOutlined />
              <span>
                <strong>{item.original_name}</strong>
                <small>{formatFileSize(item.file_size)}</small>
              </span>
              <div className="dashboard-archive-file-actions">
                <Button
                  type="text"
                  icon={<EyeOutlined />}
                  aria-label={`查看${item.original_name}`}
                  onClick={(event) => {
                    event.stopPropagation();
                    void previewFile(item);
                  }}
                />
                <Button
                  type="text"
                  icon={<DownloadOutlined />}
                  aria-label={`下载${item.original_name}`}
                  onClick={(event) => {
                    event.stopPropagation();
                    void downloadFile(item);
                  }}
                />
                <Popconfirm
                  title="删除档案"
                  description={`确定删除“${item.original_name}”吗？`}
                  okText="删除"
                  cancelText="取消"
                  okButtonProps={{ danger: true }}
                  onConfirm={() => deleteFile(item)}
                >
                  <Button
                    type="text"
                    danger
                    loading={deletingFileId === item.id}
                    icon={<DeleteOutlined />}
                    aria-label={`删除${item.original_name}`}
                    onClick={(event) => event.stopPropagation()}
                  />
                </Popconfirm>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={`暂无${title}`} />
      )}
    </section>
  );

  const handleClose = () => {
    closePreview();
    onClose();
  };

  return (
    <>
      <Modal
        centered
        footer={null}
        open={open}
        width="min(1040px, 94vw)"
        className="dashboard-alumni-detail-modal"
        title={profile ? `${profile.name} · 校友完整信息` : '校友完整信息'}
        onCancel={handleClose}
        destroyOnHidden
      >
        <Spin spinning={loading}>
          {profile ? (
            <div className="dashboard-detail-content">
              <Descriptions bordered column={{ xs: 1, sm: 2, lg: 3 }} size="small">
                <Descriptions.Item label="姓名">{displayValue(profile.name)}</Descriptions.Item>
                <Descriptions.Item label="性别">{displayValue(profile.gender)}</Descriptions.Item>
                <Descriptions.Item label="联系电话">{displayValue(profile.mobile)}</Descriptions.Item>
                <Descriptions.Item label="邮箱">{displayValue(profile.email)}</Descriptions.Item>
                <Descriptions.Item label="年级">{displayValue(profile.grade)}</Descriptions.Item>
                <Descriptions.Item label="班级">{displayValue(profile.class_name)}</Descriptions.Item>
                <Descriptions.Item label="届数">{displayValue(profile.cohort)}</Descriptions.Item>
                <Descriptions.Item label="专业">{displayValue(profile.major)}</Descriptions.Item>
                <Descriptions.Item label="培养方式">{displayValue(profile.training_mode)}</Descriptions.Item>
                <Descriptions.Item label="辅导员">{displayValue(profile.counselor)}</Descriptions.Item>
                <Descriptions.Item label="导师">{displayValue(profile.mentor)}</Descriptions.Item>
                <Descriptions.Item label="行业">{displayValue(profile.industry)}</Descriptions.Item>
                <Descriptions.Item label="职务">{displayValue(profile.position)}</Descriptions.Item>
                <Descriptions.Item label="工作单位" span={3}>{displayValue(profile.work_unit)}</Descriptions.Item>
                <Descriptions.Item label="通讯地址" span={3}>{displayValue(profile.mailing_address)}</Descriptions.Item>
                {profile.remark !== undefined ? (
                  <Descriptions.Item label="备注" span={3}>{displayValue(profile.remark)}</Descriptions.Item>
                ) : null}
                <Descriptions.Item label="状态">
                  {profile.status === 'active' ? '正常' : displayValue(profile.status)}
                </Descriptions.Item>
                <Descriptions.Item label="创建时间">{displayTime(profile.created_at)}</Descriptions.Item>
                <Descriptions.Item label="更新时间">{displayTime(profile.updated_at)}</Descriptions.Item>
              </Descriptions>

              <Spin spinning={filesLoading}>
                <div className="dashboard-archive-actions">
                  <Button
                    icon={<IdcardOutlined />}
                    type={activeArchive === 'academic_record' ? 'primary' : 'default'}
                    onClick={() =>
                      setActiveArchive((current) =>
                        current === 'academic_record' ? null : 'academic_record',
                      )
                    }
                  >
                    学籍档案
                    <span>{files?.academic_record.length || 0}</span>
                  </Button>
                  <Button
                    icon={<FileTextOutlined />}
                    type={activeArchive === 'degree_archive' ? 'primary' : 'default'}
                    onClick={() =>
                      setActiveArchive((current) =>
                        current === 'degree_archive' ? null : 'degree_archive',
                      )
                    }
                  >
                    学位档案
                    <span>{files?.degree_archive.length || 0}</span>
                  </Button>
                </div>
                {activeArchive === 'academic_record'
                  ? renderFiles('学籍档案', files?.academic_record || [], 'academic_record')
                  : null}
                {activeArchive === 'degree_archive'
                  ? renderFiles('学位档案', files?.degree_archive || [], 'degree_archive')
                  : null}
              </Spin>
            </div>
          ) : (
            <div className="dashboard-detail-placeholder">正在读取校友完整信息...</div>
          )}
        </Spin>
      </Modal>

      <Modal
        centered
        footer={null}
        open={Boolean(previewItem)}
        width="min(1120px, 96vw)"
        className="dashboard-file-preview-modal"
        title={previewItem?.original_name || '档案预览'}
        onCancel={closePreview}
        destroyOnHidden
      >
        <Spin spinning={previewLoading}>
          <div className="dashboard-file-preview">
            {previewUrl && previewItem && getMimeType(previewItem).startsWith('image/') ? (
              <img src={previewUrl} alt={previewItem.original_name} />
            ) : null}
            {previewUrl && previewItem && getMimeType(previewItem) === 'application/pdf' ? (
              <iframe src={previewUrl} title={previewItem.original_name} />
            ) : null}
          </div>
        </Spin>
      </Modal>
    </>
  );
}
