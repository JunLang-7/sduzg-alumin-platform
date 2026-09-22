import { useCallback, useEffect, useState } from 'react';
import { CheckOutlined, EyeOutlined, FileTextOutlined } from '@ant-design/icons';
import {
  App,
  Button,
  Descriptions,
  Drawer,
  Empty,
  Input,
  List,
  Modal,
  Space,
  Table,
  Tag,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { historyApi } from '../../api/history';
import type {
  HistoryAttachment,
  HistoryContribution,
  HistoryReviewAction,
} from '../../types/history';
import { PageHeader } from '../../components/PageHeader';
import './history-review.css';

const actionText: Record<HistoryReviewAction, string> = {
  approve: '通过',
  return: '退回',
  reject: '驳回',
};

const formatDate = (value?: string) => {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return date
    .toLocaleDateString('zh-CN', {
      timeZone: 'Asia/Shanghai',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    })
    .replace(/\//g, '-');
};

export function HistoryReviewPage() {
  const { message } = App.useApp();
  const [items, setItems] = useState<HistoryContribution[]>([]);
  const [active, setActive] = useState<HistoryContribution | null>(null);
  const [attachments, setAttachments] = useState<HistoryAttachment[]>([]);
  const [loading, setLoading] = useState(false);
  const load = useCallback(async () => {
    setLoading(true);
    try {
      setItems(await historyApi.listPending());
    } catch (e) {
      message.error(e instanceof Error ? e.message : '待审资料加载失败');
    } finally {
      setLoading(false);
    }
  }, [message]);
  useEffect(() => {
    void load();
  }, [load]);
  const open = async (item: HistoryContribution) => {
    setActive(item);
    setAttachments([]);
    try {
      setAttachments(await historyApi.listReviewAttachments(item.id));
    } catch (e) {
      message.error(e instanceof Error ? e.message : '附件加载失败');
    }
  };
  const review = (action: HistoryReviewAction) => {
    let comment = '';
    Modal.confirm({
      title: `${actionText[action]}投稿`,
      content:
        action === 'approve' ? (
          '通过后将更新正式词条。'
        ) : (
          <Input.TextArea
            placeholder="请填写处理说明"
            onChange={(e) => {
              comment = e.target.value;
            }}
          />
        ),
      onOk: async () => {
        if (action !== 'approve' && !comment.trim())
          return Promise.reject(new Error('请填写处理说明'));
        await historyApi.review(active!.id, action, comment);
        message.success('处理成功');
        setActive(null);
        await load();
      },
    });
  };
  const columns: ColumnsType<HistoryContribution> = [
    { title: '词条', dataIndex: 'title' },
    {
      title: '提交时间',
      width: 120,
      render: (_, item) => formatDate(item.submitted_at || item.updated_at),
    },
    {
      title: '操作',
      width: 100,
      className: 'history-review__action-column',
      render: (_, item) => (
        <Button
          className="history-review__review-button"
          type="link"
          icon={<EyeOutlined />}
          onClick={() => void open(item)}
        >
          审核
        </Button>
      ),
    },
  ];
  return (
    <section>
      <PageHeader title="院史领域待审" description="仅显示您有管理权限的培养类别投稿" />
      <Table
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={items}
        locale={{ emptyText: <Empty description="暂无待审投稿" /> }}
      />
      <Drawer
        open={Boolean(active)}
        onClose={() => setActive(null)}
        width={720}
        title="审核投稿"
        extra={
          <Space>
            <Button danger onClick={() => review('reject')}>
              驳回
            </Button>
            <Button onClick={() => review('return')}>退回补充</Button>
            <Button type="primary" icon={<CheckOutlined />} onClick={() => review('approve')}>
              通过
            </Button>
          </Space>
        }
      >
        {active && (
          <>
            <Descriptions column={1} bordered size="small">
              <Descriptions.Item label="词条">{active.title}</Descriptions.Item>
              <Descriptions.Item label="修改说明">{active.change_note || '—'}</Descriptions.Item>
              <Descriptions.Item label="正文">{active.content}</Descriptions.Item>
              <Descriptions.Item label="资料来源">{active.source_note}</Descriptions.Item>
            </Descriptions>
            {attachments.length > 0 && (
              <>
                <h3>图片和扫描件</h3>
                <List
                  dataSource={attachments}
                  renderItem={(file) => (
                    <List.Item>
                      <List.Item.Meta
                        avatar={<FileTextOutlined />}
                        title={file.original_name}
                        description={`${file.description}；来源：${file.source_note}；授权：${file.rights_note}`}
                      />
                      <Button
                        type="link"
                        icon={<EyeOutlined />}
                        onClick={async () => {
                          try {
                            const url = await historyApi.previewAttachment(active.id, file.id);
                            window.open(url, '_blank', 'noopener,noreferrer');
                          } catch (error) {
                            message.error(error instanceof Error ? error.message : '附件预览失败');
                          }
                        }}
                      >
                        预览
                      </Button>
                      <Tag color={file.consent_confirmed ? 'green' : 'red'}>
                        {file.consent_confirmed ? '已确认授权' : '未确认授权'}
                      </Tag>
                    </List.Item>
                  )}
                />
              </>
            )}
          </>
        )}
      </Drawer>
    </section>
  );
}
