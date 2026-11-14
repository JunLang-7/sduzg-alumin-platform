import { useEffect, useMemo, useState } from 'react';
import { ArrowLeftOutlined, DownloadOutlined, FileOutlined } from '@ant-design/icons';
import {
  Button,
  Card,
  Descriptions,
  Empty,
  List,
  Space,
  Spin,
  Tag,
  Typography,
  message,
} from 'antd';
import { useNavigate, useParams } from 'react-router-dom';
import { alumniApi } from '../../api/alumni';
import { AuditOperationHistoryCard } from '../../components/AuditOperationHistoryCard';
import { PageHeader } from '../../components/PageHeader';
import { useAuthStore } from '../../store/authStore';
import { canManageFiles, canReadSensitive } from '../../utils/access';
import { TRAINING_LEVEL_LABELS } from '../../types/alumni';
import type {
  AlumniEnrollment,
  AlumniFileItem,
  AlumniFileListResponse,
  AlumniProfile,
  AlumniFileType,
} from '../../types/alumni';

const FILE_TYPE_LABELS: Record<AlumniFileType, string> = {
  degree_archive: '学位档案',
  academic_record: '学籍档案',
};

function isHidden(value: unknown, isRestrictedViewer: boolean): boolean {
  return isRestrictedViewer && value == null;
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function AlumniDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const [loading, setLoading] = useState(false);
  const [profile, setProfile] = useState<AlumniProfile | null>(null);
  const [files, setFiles] = useState<AlumniFileListResponse | null>(null);
  const [filesLoading, setFilesLoading] = useState(false);

  const isOwnAlumniProfile =
    user?.role === 'alumni' && user.alumni_id !== undefined && String(user.alumni_id) === id;
  const isRestrictedViewer =
    user?.role === 'alumni' ? !isOwnAlumniProfile : !canReadSensitive(user);
  const canViewFiles = canManageFiles(user);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    alumniApi
      .detail(id)
      .then(setProfile)
      .catch((error: Error) => message.error(error.message || '校友详情加载失败'))
      .finally(() => setLoading(false));
  }, [id]);

  useEffect(() => {
    if (!id || !canViewFiles) return;
    setFilesLoading(true);
    alumniApi
      .listFiles(Number(id))
      .then(setFiles)
      .catch(() => {
        // 无附件权限或文件服务未启用时保持空列表。
        setFiles(null);
      })
      .finally(() => setFilesLoading(false));
  }, [canViewFiles, id]);

  const filesByEnrollment = useMemo(() => {
    const map = new Map<
      number,
      { degree_archive: AlumniFileItem[]; academic_record: AlumniFileItem[] }
    >();
    (files?.enrollments || []).forEach((group) => {
      map.set(group.enrollment_id, {
        degree_archive: group.degree_archive,
        academic_record: group.academic_record,
      });
    });
    return map;
  }, [files]);

  const handleDownload = async (enrollment: AlumniEnrollment, file: AlumniFileItem) => {
    if (!profile) return;
    try {
      await alumniApi.downloadFile(profile.id, enrollment.id, file.id);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '下载失败');
    }
  };

  const renderFileSection = (enrollment: AlumniEnrollment, fileType: AlumniFileType) => {
    const items = filesByEnrollment.get(enrollment.id)?.[fileType] ?? [];
    return (
      <Card
        size="small"
        title={
          <Space>
            <Tag color={fileType === 'degree_archive' ? 'blue' : 'green'}>
              {FILE_TYPE_LABELS[fileType]}
            </Tag>
            <span className="text-secondary">{items.length} 个文件</span>
          </Space>
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
                    onClick={() => void handleDownload(enrollment, item)}
                  >
                    下载
                  </Button>,
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
    <div className="alumni-detail-page audit-page">
      <PageHeader
        title="校友详情"
        extra={
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate(-1)}>
            返回
          </Button>
        }
      />
      <Card className="tool-card alumni-profile-card" title="校友档案">
        <Spin spinning={loading}>
          {profile ? (
            <Descriptions bordered column={{ xs: 1, md: 2, xl: 3 }}>
              <Descriptions.Item label="姓名">{profile.name}</Descriptions.Item>
              <Descriptions.Item label="性别">{profile.gender || '-'}</Descriptions.Item>
              <Descriptions.Item label="手机号">
                {isHidden(profile.mobile, isRestrictedViewer) ? '***' : profile.mobile || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="邮箱">
                {isHidden(profile.email, isRestrictedViewer) ? '***' : profile.email || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="行业">{profile.industry || '-'}</Descriptions.Item>
              <Descriptions.Item label="工作单位">
                {isHidden(profile.work_unit, isRestrictedViewer) ? '***' : profile.work_unit || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="职务">
                {isHidden(profile.position, isRestrictedViewer) ? '***' : profile.position || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="通讯地址" span={3}>
                {isHidden(profile.mailing_address, isRestrictedViewer)
                  ? '***'
                  : profile.mailing_address || '-'}
              </Descriptions.Item>
            </Descriptions>
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} />
          )}
        </Spin>
      </Card>

      {profile && (
        <>
          <div className="alumni-enrollment-section">
            <Typography.Title level={4}>学籍信息</Typography.Title>
            {(profile.enrollments || []).length === 0 ? (
              <Card className="tool-card">
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无可访问学籍" />
              </Card>
            ) : (
              <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                {(profile.enrollments || []).map((enrollment) => {
                  const level = TRAINING_LEVEL_LABELS[enrollment.training_level] || '待核验';
                  return (
                    <Card
                      key={enrollment.id}
                      className="tool-card alumni-enrollment-card"
                      title={
                        <Space>
                          <span>{level}学籍</span>
                          <Tag
                            color={enrollment.training_level === 'unverified' ? 'orange' : 'blue'}
                          >
                            {enrollment.grade ? `${enrollment.grade}级` : '年级待补'}
                          </Tag>
                          {enrollment.data_domain_name ? (
                            <Tag>{enrollment.data_domain_name}</Tag>
                          ) : null}
                        </Space>
                      }
                    >
                      <Descriptions bordered column={{ xs: 1, md: 2, xl: 3 }} size="small">
                        <Descriptions.Item label="培养层次">{level}</Descriptions.Item>
                        <Descriptions.Item label="培养类别">
                          {enrollment.data_domain_name || '-'}
                        </Descriptions.Item>
                        <Descriptions.Item label="年级">
                          {enrollment.grade || '-'}
                        </Descriptions.Item>
                        <Descriptions.Item label="班级">
                          {enrollment.class_name || '-'}
                        </Descriptions.Item>
                        <Descriptions.Item label="届数">
                          {enrollment.cohort || '-'}
                        </Descriptions.Item>
                        <Descriptions.Item label="专业">
                          {enrollment.major || '-'}
                        </Descriptions.Item>
                        <Descriptions.Item label="培养方式">
                          {enrollment.training_mode || '-'}
                        </Descriptions.Item>
                        <Descriptions.Item label="辅导员">
                          {enrollment.counselor || '-'}
                        </Descriptions.Item>
                        <Descriptions.Item label="导师">
                          {enrollment.mentor || '-'}
                        </Descriptions.Item>
                        {enrollment.remark ? (
                          <Descriptions.Item label="管理员备注" span={3}>
                            {enrollment.remark}
                          </Descriptions.Item>
                        ) : null}
                      </Descriptions>
                      <Typography.Title level={5} style={{ marginTop: 16 }}>
                        学位档案与学籍档案
                      </Typography.Title>
                      {canViewFiles ? (
                        <Spin spinning={filesLoading}>
                          <Space direction="vertical" style={{ width: '100%' }} size="small">
                            {renderFileSection(enrollment, 'degree_archive')}
                            {renderFileSection(enrollment, 'academic_record')}
                          </Space>
                        </Spin>
                      ) : (
                        <div className="empty-hint">当前账号无档案文件查看权限</div>
                      )}
                    </Card>
                  );
                })}
              </Space>
            )}
          </div>
          <AuditOperationHistoryCard alumniId={profile.id} />
        </>
      )}
    </div>
  );
}
