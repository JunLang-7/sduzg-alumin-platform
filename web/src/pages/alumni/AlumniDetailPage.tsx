import { useEffect, useState } from 'react';
import { ArrowLeftOutlined } from '@ant-design/icons';
import { Button, Card, Descriptions, Empty, Spin, message } from 'antd';
import { useNavigate, useParams } from 'react-router-dom';
import { alumniApi } from '../../api/alumni';
import { AlumniFilesCard } from '../../components/AlumniFilesCard';
import { PageHeader } from '../../components/PageHeader';
import type { AlumniProfile } from '../../types/alumni';

export function AlumniDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [profile, setProfile] = useState<AlumniProfile | null>(null);

  useEffect(() => {
    if (!id) {
      return;
    }

    setLoading(true);
    alumniApi
      .detail(id)
      .then(setProfile)
      .catch((error: Error) => message.error(error.message || '校友详情加载失败'))
      .finally(() => setLoading(false));
  }, [id]);

  return (
    <>
      <PageHeader
        title="校友详情"
        extra={
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate(-1)}>
            返回
          </Button>
        }
      />
      <Card className="tool-card">
        <Spin spinning={loading}>
          {profile ? (
            <Descriptions bordered column={{ xs: 1, md: 2, xl: 3 }}>
              <Descriptions.Item label="姓名">{profile.name}</Descriptions.Item>
              <Descriptions.Item label="性别">{profile.gender || '-'}</Descriptions.Item>
              <Descriptions.Item label="手机号">{profile.mobile || '-'}</Descriptions.Item>
              <Descriptions.Item label="年级">{profile.grade}</Descriptions.Item>
              <Descriptions.Item label="班级">{profile.class_name || '-'}</Descriptions.Item>
              <Descriptions.Item label="届数">{profile.cohort || '-'}</Descriptions.Item>
              <Descriptions.Item label="专业">{profile.major || '-'}</Descriptions.Item>
              <Descriptions.Item label="培养方式">{profile.training_mode || '-'}</Descriptions.Item>
              <Descriptions.Item label="辅导员">{profile.counselor || '-'}</Descriptions.Item>
              <Descriptions.Item label="导师">{profile.mentor || '-'}</Descriptions.Item>
              <Descriptions.Item label="行业">{profile.industry || '-'}</Descriptions.Item>
              <Descriptions.Item label="工作单位">{profile.work_unit || '-'}</Descriptions.Item>
              <Descriptions.Item label="职务">{profile.position || '-'}</Descriptions.Item>
              <Descriptions.Item label="通讯地址" span={3}>
                {profile.mailing_address || '-'}
              </Descriptions.Item>
            </Descriptions>
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} />
          )}
        </Spin>
      </Card>

      {profile && (
        <AlumniFilesCard alumniId={profile.id} />
      )}
    </>
  );
}
