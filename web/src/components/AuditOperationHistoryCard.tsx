import { useCallback, useEffect, useMemo, useState } from 'react';
import { EyeOutlined, HistoryOutlined } from '@ant-design/icons';
import { Button, Card, Empty, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { auditApi } from '../api/audit';
import { AuditOperationDetailDrawer } from './AuditOperationDetailDrawer';
import type { AuditOperation } from '../types/audit';
import {
  buildAlumniHistoryQuery,
  formatAuditDateTime,
  getAuditActionColor,
  getAuditActionLabel,
} from '../utils/auditHistory';

interface Props {
  alumniId: number;
}

const historyPageSize = 20;

export function AuditOperationHistoryCard({ alumniId }: Props) {
  const [items, setItems] = useState<AuditOperation[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [selectedRecord, setSelectedRecord] = useState<AuditOperation | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const loadHistory = useCallback(async () => {
    setLoading(true);
    try {
      const query = buildAlumniHistoryQuery(alumniId, page, historyPageSize);
      const result = await auditApi.list(query);
      setItems(result.items || []);
      setTotal(result.total || 0);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '校友操作历史加载失败');
    } finally {
      setLoading(false);
    }
  }, [alumniId, page]);

  useEffect(() => {
    setPage(1);
  }, [alumniId]);

  useEffect(() => {
    void loadHistory();
  }, [loadHistory]);

  const openDetails = async (record: AuditOperation) => {
    setSelectedRecord(record);
    setDetailLoading(true);
    try {
      setSelectedRecord(await auditApi.detail(record.id));
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '操作详情加载失败');
      setSelectedRecord(null);
    } finally {
      setDetailLoading(false);
    }
  };

  const columns = useMemo<ColumnsType<AuditOperation>>(
    () => [
      {
        title: '时间',
        dataIndex: 'created_at',
        width: 168,
        render: (value: string) => formatAuditDateTime(value),
      },
      {
        title: '操作人',
        dataIndex: 'operator',
        width: 170,
        render: (_value, record) => (
          <span className="audit-table-person">
            <strong>{record.operator}</strong>
            <Typography.Text type="secondary">
              {record.operator_role_label || '操作人'}
            </Typography.Text>
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
        title: '类型',
        dataIndex: 'action',
        width: 100,
        render: (value: string) => (
          <Tag color={getAuditActionColor(value)}>{getAuditActionLabel(value)}</Tag>
        ),
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
    ],
    [],
  );

  return (
    <>
      <Card
        className="tool-card audit-table-card audit-alumni-history-card"
        title={
          <span>
            <HistoryOutlined /> 操作历史{' '}
            <Typography.Text type="secondary">共 {total} 条</Typography.Text>
          </span>
        }
      >
        <Table<AuditOperation>
          rowKey="id"
          loading={loading}
          size="middle"
          columns={columns}
          dataSource={items}
          scroll={{ x: 660 }}
          pagination={{
            current: page,
            pageSize: historyPageSize,
            total,
            showSizeChanger: false,
            showTotal: (count, range) => `第 ${range[0]}-${range[1]} 条 / 共 ${count} 条`,
          }}
          onChange={(pagination) => setPage(pagination.current || 1)}
          locale={{ emptyText: <Empty description="暂无操作历史" /> }}
        />
      </Card>

      <AuditOperationDetailDrawer
        open={Boolean(selectedRecord)}
        loading={detailLoading}
        record={selectedRecord}
        onClose={() => setSelectedRecord(null)}
      />
    </>
  );
}
