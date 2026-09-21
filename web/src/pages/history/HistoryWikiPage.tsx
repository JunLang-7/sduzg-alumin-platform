import {
  EditOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import { App, Button, Empty, Input, List, Space, Tag } from 'antd';
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { historyApi } from '../../api/history';
import { useAuthStore } from '../../store/authStore';
import type { HistoryContribution, HistoryEntry } from '../../types/history';
import { historyContributionStatusColor, historyContributionStatusText } from './historyState';
import './history-wiki.css';

const displayTitle = (title: string) => title.replace(/\s*v\d+$/i, '');

export function HistoryWikiPage() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const user = useAuthStore((state) => state.user);
  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin';
  const [entries, setEntries] = useState<HistoryEntry[]>([]);
  const [mine, setMine] = useState<HistoryContribution[]>([]);
  const [active, setActive] = useState<HistoryEntry | null>(null);
  const [keyword, setKeyword] = useState('');
  const [searched, setSearched] = useState(false);
  const [directoryOpen, setDirectoryOpen] = useState(true);
  const load = async (search = keyword) => {
    try {
      const [items, contributions] = await Promise.all([
        historyApi.listEntries(search.trim() || undefined),
        user?.role === 'alumni' ? historyApi.listMine() : Promise.resolve([]),
      ]);
      setEntries(items);
      setMine(contributions);
      setActive((current) => items.find((item) => item.id === current?.id) ?? items[0] ?? null);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '院史内容加载失败');
    }
  };
  useEffect(() => {
    void load(''); // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.role]);
  const openEditor = (mode: 'create' | 'contribute' | 'admin-edit') => {
    if (mode !== 'create' && !active) return;
    const query = new URLSearchParams({ mode });
    if (active) query.set('entryId', String(active.id));
    navigate(`/history/editor?${query.toString()}`);
  };
  const search = () => {
    setSearched(true);
    void load();
  };
  return (
    <section
      className={`history-page ${directoryOpen ? '' : 'history-page--directory-collapsed'} ${user?.role !== 'alumni' ? 'history-page--without-mine' : ''}`}
    >
      <header className="history-page__header">
        <div>
          <h1>院史共编</h1>
          <p>查看已发布词条和相关资料</p>
        </div>
        <Space wrap>
          <Input
            prefix={<SearchOutlined />}
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            onPressEnter={search}
            placeholder="搜索词条标题或正文"
            allowClear
            onClear={() => {
              setSearched(false);
              void load('');
            }}
          />
          <Button onClick={search}>搜索</Button>
          {user?.role === 'alumni' && (
            <Button type="primary" icon={<EditOutlined />} onClick={() => openEditor('create')}>
              新建词条
            </Button>
          )}
          {isAdmin && active && (
            <Button type="primary" icon={<EditOutlined />} onClick={() => openEditor('admin-edit')}>
              更改词条
            </Button>
          )}
        </Space>
      </header>
      {searched && (
        <div className="history-page__search-result">
          {entries.length
            ? `搜索“${keyword.trim()}”，找到 ${entries.length} 个词条`
            : `未找到与“${keyword.trim()}”相关的正式词条`}
        </div>
      )}
      <main className="history-page__content">
        <aside className="history-page__directory">
          <Button
            type="text"
            className="history-page__directory-toggle"
            icon={directoryOpen ? <MenuFoldOutlined /> : <MenuUnfoldOutlined />}
            onClick={() => setDirectoryOpen((open) => !open)}
            aria-label="展开或收起院史目录"
          />
          {directoryOpen && (
            <>
              <strong>院史目录</strong>
              <small>{entries.length} 个词条</small>
              <List
                size="small"
                dataSource={entries}
                locale={{
                  emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无词条" />,
                }}
                renderItem={(entry) => (
                  <List.Item
                    className={entry.id === active?.id ? 'active' : ''}
                    onClick={() => setActive(entry)}
                  >
                    {displayTitle(entry.title)}
                  </List.Item>
                )}
              />
            </>
          )}
        </aside>
        <article>
          <Tag color="red">正式词条</Tag>
          <h2>{active ? displayTitle(active.title) : '暂无已发布词条'}</h2>
          {active?.summary && <p className="history-page__summary">{active.summary}</p>}
          <div className="history-page__meta">
            最近更新：{active ? new Date(active.updated_at).toLocaleDateString('zh-CN') : '—'}
          </div>
          <div className="history-page__article-actions">
            {user?.role === 'alumni' && active && (
              <Button onClick={() => openEditor('contribute')}>补充修改</Button>
            )}
          </div>
          <h3>词条正文</h3>
          <p className="history-page__body">{active?.content ?? '暂时没有内容'}</p>
          <h3>参考资料</h3>
          <p className="history-page__body">{active?.source_note || '暂无可展示的资料来源'}</p>
        </article>
        {user?.role === 'alumni' && (
          <aside className="history-page__mine">
            <strong>我的投稿</strong>
            <List
              dataSource={mine}
              locale={{
                emptyText: <Empty description="还没有投稿" image={Empty.PRESENTED_IMAGE_SIMPLE} />,
              }}
              renderItem={(item) => (
                <List.Item>
                  <div>
                    <Tag color={historyContributionStatusColor[item.status]}>
                      {historyContributionStatusText[item.status]}
                    </Tag>
                    <b>{item.title}</b>
                    <p>{item.review_comment || item.change_note || '等待审核'}</p>
                  </div>
                </List.Item>
              )}
            />
          </aside>
        )}
      </main>
    </section>
  );
}
