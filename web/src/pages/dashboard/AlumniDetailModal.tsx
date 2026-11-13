import { Button, Descriptions, Empty, Modal, Space, Spin, Tag } from 'antd';
import { useNavigate } from 'react-router-dom';
import type { AlumniProfile } from '../../types/alumni';
import { TRAINING_LEVEL_LABELS } from '../../types/alumni';
import { useAuthStore } from '../../store/authStore';
import { canReadSensitive } from '../../utils/access';

interface AlumniDetailModalProps {
  profile: AlumniProfile | null;
  loading: boolean;
  open: boolean;
  onClose: () => void;
}

function displayValue(value?: string) {
  return value?.trim() || '未填';
}

function displaySensitiveValue(value: string | undefined, readable: boolean) {
  return readable ? displayValue(value) : '***';
}

/** 大屏快速查看：只展示个人信息与全部可访问学籍，修改入口统一在管理端。 */
export function AlumniDetailModal({ profile, loading, open, onClose }: AlumniDetailModalProps) {
  const navigate = useNavigate();
  const user = useAuthStore((state) => state.user);
  const sensitiveReadable = canReadSensitive(user);

  return (
    <Modal
      open={open}
      onCancel={onClose}
      footer={null}
      width={880}
      title="校友档案"
      destroyOnHidden
    >
      <Spin spinning={loading}>
        {profile ? (
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <Descriptions bordered column={{ xs: 1, md: 2 }} size="small">
              <Descriptions.Item label="姓名">{profile.name}</Descriptions.Item>
              <Descriptions.Item label="性别">{displayValue(profile.gender)}</Descriptions.Item>
              <Descriptions.Item label="手机号">
                {displaySensitiveValue(profile.mobile, sensitiveReadable)}
              </Descriptions.Item>
              <Descriptions.Item label="邮箱">
                {displaySensitiveValue(profile.email, sensitiveReadable)}
              </Descriptions.Item>
              <Descriptions.Item label="行业">{displayValue(profile.industry)}</Descriptions.Item>
              <Descriptions.Item label="工作单位">
                {displaySensitiveValue(profile.work_unit, sensitiveReadable)}
              </Descriptions.Item>
              <Descriptions.Item label="职务">
                {displaySensitiveValue(profile.position, sensitiveReadable)}
              </Descriptions.Item>
              <Descriptions.Item label="通讯地址">
                {displaySensitiveValue(profile.mailing_address, sensitiveReadable)}
              </Descriptions.Item>
            </Descriptions>

            <div>
              <strong>学籍信息</strong>
              <Space direction="vertical" size="small" style={{ width: '100%', marginTop: 8 }}>
                {profile.enrollments.length === 0 ? (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无可访问学籍" />
                ) : (
                  profile.enrollments.map((enrollment) => (
                    <Descriptions
                      key={enrollment.id}
                      bordered
                      size="small"
                      column={{ xs: 1, md: 2 }}
                      title={
                        <Space>
                          <Tag
                            color={enrollment.training_level === 'unverified' ? 'orange' : 'blue'}
                          >
                            {TRAINING_LEVEL_LABELS[enrollment.training_level] || '待核验'}
                          </Tag>
                          <span>{enrollment.grade ? `${enrollment.grade}级` : '年级待补'}</span>
                          {enrollment.data_domain_name ? (
                            <Tag>{enrollment.data_domain_name}</Tag>
                          ) : null}
                        </Space>
                      }
                    >
                      <Descriptions.Item label="班级">
                        {displayValue(enrollment.class_name)}
                      </Descriptions.Item>
                      <Descriptions.Item label="届数">
                        {displayValue(enrollment.cohort)}
                      </Descriptions.Item>
                      <Descriptions.Item label="专业">
                        {displayValue(enrollment.major)}
                      </Descriptions.Item>
                      <Descriptions.Item label="培养方式">
                        {displayValue(enrollment.training_mode)}
                      </Descriptions.Item>
                      <Descriptions.Item label="辅导员">
                        {displayValue(enrollment.counselor)}
                      </Descriptions.Item>
                      <Descriptions.Item label="导师">
                        {displayValue(enrollment.mentor)}
                      </Descriptions.Item>
                    </Descriptions>
                  ))
                )}
              </Space>
            </div>

            <Button
              type="primary"
              block
              onClick={() => {
                onClose();
                navigate(`/alumni/${profile.id}`);
              }}
            >
              查看完整档案与操作历史
            </Button>
          </Space>
        ) : (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} />
        )}
      </Spin>
    </Modal>
  );
}
