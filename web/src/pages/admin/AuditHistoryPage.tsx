import { useCallback, useEffect, useMemo, useState } from 'react';
import { AuditOutlined, EyeOutlined, FilterOutlined } from '@ant-design/icons';
import { Button, Card, DatePicker, Empty, Select, Space, Table, Tag, Typography, message } from 'antd';
import type { RangePickerProps } from 'antd/es/date-picker';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import { Link } from 'react-router-dom';
import { auditApi } from '../../api/audit';
import { AuditOperationDetailDrawer } from '../../components/AuditOperationDetailDrawer';
import { PageHeader } from '../../components/PageHeader';
import { AUDIT_ACTION_OPTIONS, AUDIT_SCOPE_TAGS, type AuditAction, type AuditOperation, type AuditQuery } from '../../types/audit';

const defaultPageSize = 20;

const ACTION_LABELS: Record<string, string> = Object.fromEntries(
  AUDIT_ACTION_OPTIONS.map((item) => [item.value, item.label]),
);

function actionLabel(action: string) {
  return ACTION_LABELS[action] || action || '操作';
}

function actionColor(action: string) {
  return ({ create: 'green', update: 'blue', delete: 'red', import: 'gold' } as Record<string, string>)[action];
}

function formatDateTime(value: string) {
  return value.replace('T', ' ').slice(0, 16);
}

export function AuditHistoryPage() {
  const [dateRange, setDateRange] = useState<RangePickerProps['value']>(null);
  const [scopeFilter, setScopeFilter] = useState('');
  const [actionFilter, setActionFilter] = useState<AuditAction | undefined>();
  const [page, setPage] = useState(1);
  const [data, setData] = useState<{ items: AuditOperation[]; total: number }>({ items: [], total: 0 });
  const [loading, setLoading] = useState(false);
  const [selectedRecord, setSelectedRecord] = useState<AuditOperation | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const requestQuery = useMemo<AuditQuery>(() => ({
    page,
    page_size: defaultPageSize,
    start_date: dateRange?.[0]?.format('YYYY-MM-DD'),
    end_date: dateRange?.[1]?.format('YYYY-MM-DD'),
    management_scope: scopeFilter || undefined,
    action: actionFilter,
  }), [actionFilter, dateRange, page, scopeFilter]);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const result = await auditApi.list(requestQuery);
      setData({ items: result.items || [], total: result.total || 0 });
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '操作历史加载失败');
    } finally {
      setLoading(false);
    }
  }, [requestQuery]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  const resetFilters = () => {
    setDateRange(null);
    setScopeFilter('');
    setActionFilter(undefined);
    setPage(1);
  };

  const openDetails = async (record: AuditOperation) => {
    setSelectedRecord(record);
    setDetailLoading(true);
    try {
      const detail = await auditApi.detail(record.id);
      setSelectedRecord(detail);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '操作详情加载失败');
      setSelectedRecord(null);
    } finally {
      setDetailLoading(false);
    }
  };

  const columns = useMemo<ColumnsType<AuditOperation>>(() => [
    {
      title: '时间',
      dataIndex: 'created_at',
      width: 168,
      render: (value: string) => <Typography.Text>{formatDateTime(value)}</Typography.Text>,
    },
    {
      title: '操作人',
      dataIndex: 'operator',
      width: 160,
      render: (_value, record) => (
        <span className="audit-table-person">
          <strong>{record.operator}</strong>
          <Typography.Text type="secondary">{record.operator_role_label || '操作人'}</Typography.Text>
        </span>
      ),
    },
    {
      title: '管理范围',
      dataIndex: 'management_scope',
      width: 140,
      render: (value: string) => <Tag className="audit-scope-tag">{value || '全部校友'}</Tag>,
    },
    {
      title: '校友档案',
      dataIndex: 'target_name',
      width: 220,
      render: (_value, record) => (
        <span className="audit-table-target">
          {record.target_id && record.target_type === 'alumni_profile' ? (
            <Link className="audit-table-alumni-link" to={`/alumni/${record.target_id}`}>{record.target_name}</Link>
          ) : (
            <strong>{record.target_name}</strong>
          )}
          {record.target_meta ? <Typography.Text type="secondary">{record.target_meta}</Typography.Text> : null}
        </span>
      ),
    },
    {
      title: '类型',
      dataIndex: 'action',
      width: 100,
      render: (value: string) => <Tag color={actionColor(value)}>{actionLabel(value)}</Tag>,
    },
    {
      title: '操作',
      key: 'operation',
      width: 92,
      fixed: 'right',
      render: (_value, record) => (
        <Button type="link" icon={<EyeOutlined />} onClick={() => void openDetails(record)}>
          详情
        </Button>
      ),
    },
  ], []);

  const handleTableChange = (pagination: TablePaginationConfig) => {
    setPage(pagination.current || 1);
  };

  return (
    <div className="audit-page audit-history-page">
      <PageHeader
        title="操作历史"
        description="查看管理员和校友本人对校友档案的操作记录。"
      />

      <Card
        className="tool-card audit-filter-card"
        title={<Space><FilterOutlined />筛选记录</Space>}
        extra={<Button onClick={resetFilters}>清空筛选</Button>}
      >
        <div className="audit-filter-row">
          <DatePicker.RangePicker
            className="audit-date-range"
            format="YYYY-MM-DD"
            placeholder={['开始日期', '结束日期']}
            value={dateRange}
            onChange={(value) => {
              setDateRange(value);
              setPage(1);
            }}
          />
          <Select
            allowClear
            value={scopeFilter || undefined}
            onChange={(value) => {
              setScopeFilter(value || '');
              setPage(1);
            }}
            className="audit-scope-select"
            placeholder="全部管理范围"
            options={AUDIT_SCOPE_TAGS.map((item) => ({ value: item.value, label: item.label }))}
          />
          <Select
            allowClear
            value={actionFilter}
            onChange={(value) => {
              setActionFilter(value as AuditAction | undefined);
              setPage(1);
            }}
            className="audit-action-select"
            placeholder="全部操作类型"
            options={AUDIT_ACTION_OPTIONS}
          />
        </div>
      </Card>

      <Card
        className="tool-card audit-table-card audit-records-card"
        title={<Space><AuditOutlined />操作历史 <Typography.Text type="secondary">共 {data.total} 条</Typography.Text></Space>}
      >
        <Table<AuditOperation>
          rowKey="id"
          loading={loading}
          size="middle"
          columns={columns}
          dataSource={data.items}
          scroll={{ x: 980 }}
          pagination={{
            current: page,
            pageSize: defaultPageSize,
            total: data.total,
            showSizeChanger: false,
            showTotal: (total, range) => `第 ${range[0]}-${range[1]} 条 / 共 ${total} 条`,
          }}
          onChange={handleTableChange}
          locale={{ emptyText: <Empty description="暂无操作历史" /> }}
        />
      </Card>

      <AuditOperationDetailDrawer
        open={Boolean(selectedRecord)}
        loading={detailLoading}
        record={selectedRecord}
        onClose={() => setSelectedRecord(null)}
      />
    </div>
  );
}
