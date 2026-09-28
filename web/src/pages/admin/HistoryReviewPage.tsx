import { useCallback, useEffect, useState } from 'react';
import { ArrowLeftOutlined, CheckOutlined, EyeOutlined, FileTextOutlined } from '@ant-design/icons';
import { App, Button, Empty, Input, List, Modal, Space, Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useNavigate, useParams } from 'react-router-dom';
import { historyApi } from '../../api/history';
import type {
  HistoryAttachment,
  HistoryContribution,
  HistoryReviewAction,
} from '../../types/history';
import { PageHeader } from '../../components/PageHeader';
import { HistoryContent } from '../history/HistoryContent';
import { extractToc } from '../history/toc';
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
  return date.toLocaleDateString('zh-CN', { timeZone: 'Asia/Shanghai' }).replace(/\//g, '-');
};

export function HistoryReviewPage() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const { id } = useParams();
  const [items, setItems] = useState<HistoryContribution[]>([]);
  const [attachments, setAttachments] = useState<HistoryAttachment[]>([]);
  const [loading, setLoading] = useState(false);
  const active = id ? items.find((item) => item.id === Number(id)) : undefined;
  const activeID = active?.id;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setItems(await historyApi.listPending());
    } catch (error) {
      message.error(error instanceof Error ? error.message : '待审资料加载失败');
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    if (!activeID) return;
    let cancelled = false;
    void historyApi.listReviewAttachments(activeID).then(
      (files) => {
        if (!cancelled) setAttachments(files);
      },
      (error) => {
        if (!cancelled) message.error(error instanceof Error ? error.message : '附件加载失败');
      },
    );
    return () => {
      cancelled = true;
    };
  }, [activeID, message]);

  const review = (action: HistoryReviewAction) => {
    if (!active) return;
    let comment = '';
    Modal.confirm({
      title: `${actionText[action]}投稿`,
      content:
        action === 'approve' ? (
          '通过后将更新正式词条'
        ) : (
          <Input.TextArea
            placeholder="请填写处理说明"
            onChange={(event) => {
              comment = event.target.value;
            }}
          />
        ),
      onOk: async () => {
        if (action !== 'approve' && !comment.trim()) {
          message.warning('请填写处理说明');
          return Promise.reject(new Error('请填写处理说明'));
        }
        await historyApi.review(active.id, action, comment);
        message.success('处理成功');
        navigate('/admin/history/reviews');
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
          onClick={() => navigate(`/admin/history/reviews/${item.id}`)}
        >
          审核
        </Button>
      ),
    },
  ];

  if (!id)
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
      </section>
    );

  return (
    <section className="history-review">
      <header className="history-review__header">
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/admin/history/reviews')}>
          返回待审列表
        </Button>
        <Space wrap>
          <Button danger disabled={!active} onClick={() => review('reject')}>
            驳回
          </Button>
          <Button disabled={!active} onClick={() => review('return')}>
            退回补充
          </Button>
          <Button
            type="primary"
            icon={<CheckOutlined />}
            disabled={!active}
            onClick={() => review('approve')}
          >
            通过
          </Button>
        </Space>
      </header>
      {active ? (
        <article className="history-review__article">
          <Tag color="blue">待审核词条</Tag>
          <h1>{active.title}</h1>
          {active.change_note && <p className="history-review__summary">{active.change_note}</p>}
          <div className="history-review__meta">
            提交时间：{formatDate(active.submitted_at || active.updated_at)}
          </div>
          <h2>词条正文</h2>
          <HistoryContent content={active.content} toc={extractToc(active.content)} />
          <h2>参考资料</h2>
          <div className="history-review__body">{active.source_note || '暂无资料来源'}</div>
          <h2>图片和扫描件</h2>
          <List
            dataSource={attachments}
            locale={{ emptyText: '暂无附件' }}
            renderItem={(file) => (
              <List.Item
                actions={[
                  <Button
                    key="preview"
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
                  </Button>,
                ]}
              >
                <List.Item.Meta
                  avatar={<FileTextOutlined />}
                  title={file.original_name}
                  description={`${file.description}；来源：${file.source_note}；授权：${file.rights_note}；${file.consent_confirmed ? '已确认授权' : '未确认授权'}`}
                />
              </List.Item>
            )}
          />
        </article>
      ) : loading ? (
        <Empty description="正在加载投稿" />
      ) : (
        <Empty description="投稿不在当前待审范围" />
      )}
    </section>
  );
}
