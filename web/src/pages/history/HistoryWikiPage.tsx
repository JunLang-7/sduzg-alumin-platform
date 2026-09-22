import { EditOutlined, MenuOutlined, SearchOutlined } from '@ant-design/icons';
import {
  App,
  Button,
  Descriptions,
  Drawer,
  Empty,
  Input,
  List,
  Modal,
  Popconfirm,
  Space,
  Tag,
} from 'antd';
import { useEffect, useLayoutEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { historyApi } from '../../api/history';
import { useAuthStore } from '../../store/authStore';
import type { HistoryAttachment, HistoryContribution, HistoryEntry } from '../../types/history';
import { historyContributionStatusColor, historyContributionStatusText } from './historyState';
import { extractToc, type TocItem } from './toc';
import './history-wiki.css';

const displayTitle = (title: string) => title.replace(/\s*v\d+$/i, '');
const formatHistoryDate = (value: string) => new Date(value).toLocaleDateString('zh-CN');
const contributionNote = (item: HistoryContribution) =>
  item.review_comment || item.change_note || (item.status === 'pending' ? '等待审核' : '');
const titleVersion = (title: string) => title.match(/\s+(v\d+)$/i)?.[1];

function HistoryDirectory({
  activeID,
  entries,
  onSelect,
  showHeader = true,
  showCount = true,
}: {
  activeID?: number;
  entries: HistoryEntry[];
  onSelect: (entry: HistoryEntry) => void;
  showHeader?: boolean;
  showCount?: boolean;
}) {
  return (
    <>
      {showHeader && <strong>院史目录</strong>}
      {showCount && (
        <small className={showHeader ? undefined : 'history-page__mobile-directory-count'}>
          {entries.length} 个词条
        </small>
      )}
      <List
        className="history-page__directory-list"
        size="small"
        dataSource={entries}
        locale={{
          emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无词条" />,
        }}
        renderItem={(entry) => (
          <List.Item
            className={entry.id === activeID ? 'active' : ''}
            onClick={() => onSelect(entry)}
          >
            <span>{displayTitle(entry.title)}</span>
            {titleVersion(entry.title) && <em>{titleVersion(entry.title)}</em>}
          </List.Item>
        )}
      />
    </>
  );
}

function HistoryContent({ content, toc }: { content: string; toc: TocItem[] }) {
  const headings = new Map(toc.map((item) => [item.line, item]));
  return (
    <div className="history-page__body">
      {content.split(/\r?\n/).map((line, index) => {
        const heading = headings.get(index);
        if (heading) {
          const Heading = heading.level === 2 ? 'h3' : 'h4';
          return (
            <Heading id={heading.id} key={heading.id}>
              {heading.title}
            </Heading>
          );
        }
        return line ? <p key={`${index}-${line}`}>{line}</p> : <br key={index} />;
      })}
    </div>
  );
}

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
  const [mobileDirectoryOpen, setMobileDirectoryOpen] = useState(() => window.innerWidth <= 1180);
  const [activeTocID, setActiveTocID] = useState('');
  const [selectedContribution, setSelectedContribution] = useState<HistoryContribution | null>(
    null,
  );
  const [selectedAttachments, setSelectedAttachments] = useState<HistoryAttachment[]>([]);
  const [attachmentsLoading, setAttachmentsLoading] = useState(false);
  const [preview, setPreview] = useState<{ url: string; name: string; image: boolean } | null>(
    null,
  );
  const [contentDetailOpen, setContentDetailOpen] = useState(false);
  const load = async (search = keyword) => {
    try {
      const [items, contributions] = await Promise.all([
        historyApi.listEntries(search.trim() || undefined),
        user?.role === 'alumni' || isAdmin ? historyApi.listMine() : Promise.resolve([]),
      ]);
      setEntries(items);
      setMine(
        [...contributions].sort(
          (left, right) =>
            new Date(right.updated_at).getTime() - new Date(left.updated_at).getTime(),
        ),
      );
      setActive((current) => items.find((item) => item.id === current?.id) ?? items[0] ?? null);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '院史内容加载失败');
    }
  };
  useEffect(() => {
    void load(''); // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.role]);
  useLayoutEffect(() => {
    const syncDirectoryDrawer = () => setMobileDirectoryOpen(window.innerWidth <= 1180);
    syncDirectoryDrawer();
    window.addEventListener('resize', syncDirectoryDrawer);
    return () => window.removeEventListener('resize', syncDirectoryDrawer);
  }, []);
  const toc = useMemo(() => extractToc(active?.content ?? ''), [active?.content]);
  useEffect(() => {
    setActiveTocID(toc[0]?.id ?? '');
    if (!toc.length) return;
    const observer = new IntersectionObserver(
      (observations) => {
        const current = observations.find((observation) => observation.isIntersecting);
        if (current) setActiveTocID(current.target.id);
      },
      { rootMargin: '-18% 0px -70% 0px' },
    );
    const sections = toc
      .map((item) => document.getElementById(item.id))
      .filter((element): element is HTMLElement => element !== null);
    sections.forEach((section) => observer.observe(section));
    return () => observer.disconnect();
  }, [toc]);
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
  const openContribution = async (contribution: HistoryContribution) => {
    setSelectedContribution(contribution);
    setSelectedAttachments([]);
    setAttachmentsLoading(true);
    try {
      setSelectedAttachments(await historyApi.listReviewAttachments(contribution.id));
    } catch (error) {
      message.error(error instanceof Error ? error.message : '附件加载失败');
    } finally {
      setAttachmentsLoading(false);
    }
  };
  const deleteDraft = async (contribution: HistoryContribution) => {
    try {
      await historyApi.deleteDraft(contribution.id);
      setMine((current) => current.filter((item) => item.id !== contribution.id));
      if (selectedContribution?.id === contribution.id) setSelectedContribution(null);
      message.success('草稿已删除');
    } catch (error) {
      message.error(error instanceof Error ? error.message : '草稿删除失败');
    }
  };
  const openContributionEditor = (contribution: HistoryContribution) => {
    setSelectedContribution(null);
    navigate(`/history/editor?mode=contribution-edit&contributionId=${contribution.id}`);
  };
  const canEditContribution = (contribution: HistoryContribution) =>
    contribution.status === 'draft' ||
    contribution.status === 'pending' ||
    contribution.status === 'returned';
  const selectEntry = (entry: HistoryEntry) => {
    setActive(entry);
    setMobileDirectoryOpen(false);
  };
  const scrollToSection = (item: TocItem) => {
    document.getElementById(item.id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    setActiveTocID(item.id);
  };
  return (
    <section className="history-page">
      <header className="history-page__header">
        <div>
          <h1>院史共编</h1>
          <p>查看已发布词条和相关资料</p>
        </div>
        <Space wrap>
          <Button
            className="history-page__mobile-directory-trigger"
            icon={<MenuOutlined />}
            onClick={() => setMobileDirectoryOpen(true)}
            aria-label="打开院史目录"
          >
            目录
          </Button>
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
          {(user?.role === 'alumni' || isAdmin) && (
            <Button type="primary" icon={<EditOutlined />} onClick={() => openEditor('create')}>
              新建词条
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
          <HistoryDirectory activeID={active?.id} entries={entries} onSelect={selectEntry} />
        </aside>
        <article>
          <Tag color="red">正式词条</Tag>
          <h2>{active ? displayTitle(active.title) : '暂无已发布词条'}</h2>
          {active?.summary && <p className="history-page__summary">{active.summary}</p>}
          <div className="history-page__meta">
            <span>
              最近更新：{active ? new Date(active.updated_at).toLocaleDateString('zh-CN') : '—'}
            </span>
            {user?.role === 'alumni' && active && (
              <Button onClick={() => openEditor('contribute')}>补充修改</Button>
            )}
            {isAdmin && active && (
              <Button
                type="primary"
                icon={<EditOutlined />}
                onClick={() => openEditor('admin-edit')}
              >
                编辑词条
              </Button>
            )}
          </div>
          <h3>词条正文</h3>
          <HistoryContent content={active?.content ?? '暂时没有内容'} toc={toc} />
          <h3>参考资料</h3>
          <div className="history-page__body">
            <p>{active?.source_note || '暂无可展示的资料来源'}</p>
          </div>
        </article>
        <div className="history-page__right-rail">
          <aside className="history-page__toc">
            <strong>本页目录</strong>
            {toc.length ? (
              <nav aria-label="本页目录">
                {toc.map((item) => (
                  <button
                    className={`${item.level === 3 ? 'is-subsection' : ''} ${item.id === activeTocID ? 'active' : ''}`}
                    key={item.id}
                    onClick={() => scrollToSection(item)}
                    type="button"
                  >
                    {item.title}
                  </button>
                ))}
              </nav>
            ) : (
              <small>正文暂无章节标题</small>
            )}
          </aside>
          {(user?.role === 'alumni' || isAdmin) && (
            <aside className="history-page__mine">
              <strong>我的投稿</strong>
              <List
                dataSource={mine}
                locale={{
                  emptyText: (
                    <Empty description="还没有投稿" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                  ),
                }}
                renderItem={(item) => (
                  <List.Item
                    className="history-page__mine-item"
                    onClick={() => void openContribution(item)}
                  >
                    <div className="history-page__mine-item-content">
                      <Tag color={historyContributionStatusColor[item.status]}>
                        {historyContributionStatusText[item.status]}
                      </Tag>
                      <b>{item.title}</b>
                      {contributionNote(item) && <p>{contributionNote(item)}</p>}
                      <span className="history-page__mine-item-time">
                        最后修改：{formatHistoryDate(item.updated_at)}
                      </span>
                    </div>
                  </List.Item>
                )}
              />
            </aside>
          )}
        </div>
      </main>
      <Drawer
        className="history-page__mobile-directory"
        closable
        open={mobileDirectoryOpen}
        placement="left"
        rootClassName="history-page__mobile-directory-root"
        title="院史目录"
        width={230}
        onClose={() => setMobileDirectoryOpen(false)}
      >
        <HistoryDirectory
          activeID={active?.id}
          entries={entries}
          onSelect={selectEntry}
          showHeader={false}
          showCount={false}
        />
      </Drawer>
      <Modal
        open={Boolean(selectedContribution)}
        title={<span className="history-page__contribution-title">投稿详情</span>}
        footer={null}
        onCancel={() => {
          setSelectedContribution(null);
          setContentDetailOpen(false);
        }}
        width={720}
      >
        {selectedContribution && (
          <>
            <Descriptions column={1} bordered size="small">
              <Descriptions.Item label="标题">{selectedContribution.title}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={historyContributionStatusColor[selectedContribution.status]}>
                  {historyContributionStatusText[selectedContribution.status]}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="正文">
                <div className="history-page__contribution-preview">
                  <span title={selectedContribution.content}>{selectedContribution.content}</span>
                  <Button type="link" onClick={() => setContentDetailOpen(true)}>
                    查看详情
                  </Button>
                </div>
              </Descriptions.Item>
              <Descriptions.Item label="资料来源">
                {selectedContribution.source_note || '—'}
              </Descriptions.Item>
              <Descriptions.Item label="审核意见">
                {selectedContribution.review_comment || '—'}
              </Descriptions.Item>
            </Descriptions>
            <div className="history-page__contribution-actions">
              {selectedContribution.status === 'draft' && (
                <Popconfirm
                  title="删除这份草稿？"
                  description="删除后无法恢复"
                  okText="删除"
                  cancelText="取消"
                  okButtonProps={{ danger: true }}
                  onConfirm={() => void deleteDraft(selectedContribution)}
                >
                  <Button danger>删除草稿</Button>
                </Popconfirm>
              )}
              {canEditContribution(selectedContribution) && (
                <Button type="primary" onClick={() => openContributionEditor(selectedContribution)}>
                  编辑投稿
                </Button>
              )}
            </div>
            <h3 className="history-page__contribution-title history-page__contribution-section-title">
              附件
            </h3>
            <List
              loading={attachmentsLoading}
              dataSource={selectedAttachments}
              locale={{ emptyText: '暂无附件' }}
              renderItem={(file) => (
                <List.Item>
                  <span>{file.original_name}</span>
                  <Button
                    type="link"
                    onClick={async () => {
                      try {
                        const url = await historyApi.previewAttachment(
                          selectedContribution.id,
                          file.id,
                        );
                        setPreview({
                          url,
                          name: file.original_name,
                          image: file.mime_type.startsWith('image/'),
                        });
                      } catch (error) {
                        message.error(error instanceof Error ? error.message : '附件预览失败');
                      }
                    }}
                  >
                    预览
                  </Button>
                </List.Item>
              )}
            />
          </>
        )}
      </Modal>
      <Modal
        open={contentDetailOpen && Boolean(selectedContribution)}
        title="正文详情"
        footer={null}
        onCancel={() => setContentDetailOpen(false)}
        width={760}
      >
        <p className="history-page__contribution-content">{selectedContribution?.content}</p>
      </Modal>
      <Modal
        open={Boolean(preview)}
        title={preview?.name}
        footer={null}
        onCancel={() => setPreview(null)}
        width={preview?.image ? 900 : 1000}
      >
        {preview?.image ? (
          <img className="history-page__preview-image" src={preview.url} alt={preview.name} />
        ) : (
          <iframe
            className="history-page__preview-frame"
            src={preview?.url}
            title={preview?.name}
          />
        )}
      </Modal>
    </section>
  );
}
