import { ArrowRightOutlined } from '@ant-design/icons';
import { Card, Descriptions, Divider, Drawer, Empty, Space, Tag, Typography } from 'antd';
import { Link } from 'react-router-dom';
import type { AuditOperation } from '../types/audit';
import { getAuditActionColor, getAuditActionLabel } from '../utils/auditHistory';

interface Props {
  open: boolean;
  loading?: boolean;
  record: AuditOperation | null;
  onClose: () => void;
}

const SOURCE_LABELS: Record<string, string> = {
  admin: '管理员操作',
  admin_import: '批量导入',
  alumni_self: '校友本人修改',
};

function sourceLabel(source: string) {
  return SOURCE_LABELS[source] || source || '系统操作';
}

function statusLabel(status: string) {
  return status === 'conflicted' ? '有冲突' : '已记录';
}

function statusColor(status: string) {
  return status === 'conflicted' ? 'orange' : 'green';
}

export function AuditOperationDetailDrawer({ open, loading = false, record, onClose }: Props) {
  return (
    <Drawer
      open={open}
      loading={loading}
      onClose={onClose}
      width={540}
      title="操作详情"
      destroyOnClose
    >
      {record ? (
        <div className="audit-drawer">
          <div className="audit-drawer-heading">
            <div>
              <Typography.Text type="secondary">
                {record.target_type === 'alumni_batch' ? '校友档案批量操作' : '校友档案操作'}
              </Typography.Text>
              <h2>
                {record.operator}
                {getAuditActionLabel(record.action)}了 {record.target_name}
              </h2>
              <p>{record.created_at}</p>
            </div>
          </div>

          <Descriptions size="small" column={2} bordered>
            <Descriptions.Item label="操作人">
              {record.operator} · {record.operator_role_label || '操作人'}
            </Descriptions.Item>
            <Descriptions.Item label="操作类型">
              <Tag color={getAuditActionColor(record.action)}>
                {getAuditActionLabel(record.action)}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="操作来源">{sourceLabel(record.source)}</Descriptions.Item>
            <Descriptions.Item label="管理范围">
              {record.management_scope || '全部校友'}
            </Descriptions.Item>
            <Descriptions.Item label="校友档案" span={2}>
              {record.target_id && record.target_type === 'alumni_profile' ? (
                <Link to={`/alumni/${record.target_id}`}>{record.target_name}</Link>
              ) : (
                record.target_name
              )}
              {record.target_meta ? (
                <Typography.Text type="secondary"> · {record.target_meta}</Typography.Text>
              ) : null}
            </Descriptions.Item>
            <Descriptions.Item label="记录状态" span={record.reason ? 1 : 2}>
              <Tag color={statusColor(record.status)}>{statusLabel(record.status)}</Tag>
            </Descriptions.Item>
            {record.reason ? (
              <Descriptions.Item label="操作说明">{record.reason}</Descriptions.Item>
            ) : null}
          </Descriptions>

          <Divider orientation="left">字段变化</Divider>
          {record.changes && record.changes.length > 0 ? (
            record.changes.map((change) => (
              <Card
                size="small"
                key={`${change.field_name}-${change.field_label}`}
                className="audit-drawer-diff"
              >
                <div className="audit-drawer-field-head">
                  <strong>{change.field_label}</strong>
                </div>
                <div className="audit-drawer-values">
                  <Typography.Text delete type="secondary">
                    {change.old_value || '未填写'}
                  </Typography.Text>
                  <ArrowRightOutlined />
                  <Typography.Text strong>{change.new_value || '未填写'}</Typography.Text>
                </div>
              </Card>
            ))
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无字段变化明细" />
          )}
        </div>
      ) : (
        <Space className="audit-drawer-readonly">
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="未选择操作记录" />
        </Space>
      )}
    </Drawer>
  );
}
