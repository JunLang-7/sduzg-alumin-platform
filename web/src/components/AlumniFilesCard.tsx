import { useCallback, useEffect, useState } from 'react';
import {
  DeleteOutlined,
  DownloadOutlined,
  FileAddOutlined,
  FileOutlined,
  LoadingOutlined,
} from '@ant-design/icons';
import {
  Button,
  Card,
  List,
  Popconfirm,
  Space,
  Spin,
  Tag,
  Upload,
  message,
} from 'antd';
import { alumniApi } from '../api/alumni';
import { useAuthStore } from '../store/authStore';
import type { AlumniFileItem, AlumniFileListResponse } from '../types/alumni';

const FILE_TYPE_LABELS: Record<string, string> = {
  degree_archive: '学位档案',
  academic_record: '学籍档案',
};

const FILE_TYPE_COLORS: Record<string, string> = {
  degree_archive: 'blue',
  academic_record: 'green',
};

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

interface Props {
  alumniId: number;
}

export function AlumniFilesCard({ alumniId }: Props) {
  const user = useAuthStore((s) => s.user);
  const isAdmin =
    user?.role === 'admin' || user?.role === 'super_admin';

  const [loading, setLoading] = useState(false);
  const [files, setFiles] = useState<AlumniFileListResponse | null>(null);
  const [uploading, setUploading] = useState<string | null>(null);

  const loadFiles = useCallback(async () => {
    setLoading(true);
    try {
      const data = await alumniApi.listFiles(alumniId);
      setFiles(data);
    } catch {
      // 403/404 等情况静默处理，不弹错误
    } finally {
      setLoading(false);
    }
  }, [alumniId]);

  useEffect(() => {
    void loadFiles();
  }, [loadFiles]);

  const handleUpload = async (
    fileType: 'degree_archive' | 'academic_record',
    file: File,
  ) => {
    setUploading(fileType);
    try {
      await alumniApi.uploadFile(alumniId, fileType, file);
      message.success(`${FILE_TYPE_LABELS[fileType]}上传成功`);
      await loadFiles();
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '文件上传失败');
    } finally {
      setUploading(null);
    }
  };

  const handleDelete = async (fileId: number) => {
    try {
      await alumniApi.deleteFile(alumniId, fileId);
      message.success('文件已删除');
      await loadFiles();
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '文件删除失败');
    }
  };

  const handleDownload = async (item: AlumniFileItem) => {
    try {
      await alumniApi.downloadFile(alumniId, item.id);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '下载失败');
    }
  };

  const renderFileSection = (
    fileType: 'degree_archive' | 'academic_record',
  ) => {
    const items: AlumniFileItem[] =
      fileType === 'degree_archive'
        ? files?.degree_archive ?? []
        : files?.academic_record ?? [];

    return (
      <Card
        size="small"
        title={
          <Space>
            <Tag color={FILE_TYPE_COLORS[fileType]}>
              {FILE_TYPE_LABELS[fileType]}
            </Tag>
            <span className="text-secondary">{items.length} 个文件</span>
          </Space>
        }
        extra={
          isAdmin ? (
            <Upload
              accept=".pdf,.doc,.docx,.jpg,.jpeg,.png"
              showUploadList={false}
              beforeUpload={(file) => {
                void handleUpload(fileType, file as File);
                return false;
              }}
              disabled={uploading === fileType}
            >
              <Button
                type="link"
                icon={
                  uploading === fileType ? (
                    <LoadingOutlined />
                  ) : (
                    <FileAddOutlined />
                  )
                }
                disabled={uploading === fileType}
              >
                {uploading === fileType ? '上传中...' : '上传文件'}
              </Button>
            </Upload>
          ) : null
        }
      >
        {items.length === 0 ? (
          <div className="empty-hint">暂无文件</div>
        ) : (
          <List
            size="small"
            dataSource={items}
            renderItem={(item) => (
              <List.Item
                actions={[
                  <Button
                    key="download"
                    type="link"
                    icon={<DownloadOutlined />}
                    onClick={() => handleDownload(item)}
                  >
                    下载
                  </Button>,
                  ...(isAdmin
                    ? [
                        <Popconfirm
                          key="delete"
                          title="确认删除"
                          description={`确定删除文件「${item.original_name}」？`}
                          onConfirm={() => handleDelete(item.id)}
                        >
                          <Button
                            type="link"
                            danger
                            icon={<DeleteOutlined />}
                          >
                            删除
                          </Button>
                        </Popconfirm>,
                      ]
                    : []),
                ]}
              >
                <List.Item.Meta
                  avatar={<FileOutlined />}
                  title={item.original_name}
                  description={formatFileSize(item.file_size)}
                />
              </List.Item>
            )}
          />
        )}
      </Card>
    );
  };

  return (
    <Spin spinning={loading}>
      <Card title="学位档案与学籍档案" className="tool-card">
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          {renderFileSection('degree_archive')}
          {renderFileSection('academic_record')}
        </Space>
      </Card>
    </Spin>
  );
}
