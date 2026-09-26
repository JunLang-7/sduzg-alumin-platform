import { Descriptions, Divider, Drawer, Empty, Space, Tag, Typography } from 'antd';
import { Link } from 'react-router-dom';
import type { AuditOperation } from '../types/audit';
import {
  getAuditActionColor,
  getAuditActionLabel,
  getAuditBatchVisibleFields,
} from '../utils/auditHistory';

interface Props {
  open: boolean;
  loading?: boolean;
  record: AuditOperation | null;
  sensitiveReadable?: boolean;
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

function renderBatchImportedRecords(record: AuditOperation, sensitiveReadable: boolean) {
  if (!record.batch_created_alumni?.length) {
    return null;
  }

  const visibleFields = getAuditBatchVisibleFields(
    record.batch_import_fields,
    sensitiveReadable,
  ).filter((field) => field.field_name !== 'name');
  const hiddenSensitiveCount = record.batch_hidden_sensitive_count || 0;

  return (
    <div className="audit-batch-records-scroll">
      <table className="audit-batch-records-table">
        <thead>
          <tr>
            <th scope="col">姓名</th>
            {visibleFields.map((field) => (
              <th scope="col" key={field.field_name}>
                {field.field_label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {record.batch_created_alumni.map((alumnus) => (
            <tr key={alumnus.id}>
              <td>
                <Link to={`/alumni/${alumnus.id}`}>{alumnus.name}</Link>
              </td>
              {visibleFields.map((field) => (
                <td key={field.field_name}>{alumnus.field_values?.[field.field_name] || '—'}</td>
              ))}
            </tr>
          ))}
        </tbody>
        {hiddenSensitiveCount > 0 ? (
          <tfoot>
            <tr>
              <td colSpan={visibleFields.length + 1}>
                {sensitiveReadable
                  ? `已授权查看 ${visibleFields.filter((field) => field.sensitive).length} 项敏感字段`
                  : `隐私字段已隐藏 ${hiddenSensitiveCount} 项，当前账号无权查看具体内容`}
              </td>
            </tr>
          </tfoot>
        ) : null}
      </table>
    </div>
  );
}

export function AuditOperationDetailDrawer({
  open,
  loading = false,
  record,
  sensitiveReadable = false,
  onClose,
}: Props) {
  return (
    <Drawer
      open={open}
      loading={loading}
      onClose={onClose}
      rootClassName="audit-page"
      width={record?.target_type === 'alumni_batch' ? 'min(1180px, 92vw)' : 540}
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
          {record.target_type === 'alumni_batch' && record.batch_created_alumni?.length ? (
            renderBatchImportedRecords(record, sensitiveReadable)
          ) : record.changes && record.changes.length > 0 ? (
            <table className="audit-change-table">
              <thead>
                <tr>
                  <th scope="col">字段</th>
                  <th scope="col">原值</th>
                  <th scope="col">新值</th>
                </tr>
              </thead>
              <tbody>
                {record.changes.map((change) => (
                  <tr key={`${change.field_name}-${change.field_label}`}>
                    <th scope="row">{change.field_label}</th>
                    <td>
                      <Typography.Text delete type="secondary">
                        {change.old_value || '未填写'}
                      </Typography.Text>
                    </td>
                    <td>
                      <Typography.Text strong>{change.new_value || '未填写'}</Typography.Text>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
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
