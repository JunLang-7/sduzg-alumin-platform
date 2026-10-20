import { DeleteOutlined, EditOutlined, MenuOutlined, SearchOutlined } from '@ant-design/icons';
import { App, Button, Drawer, Empty, Input, List, Modal, Popconfirm, Space, Spin, Tag } from 'antd';
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { historyApi } from '../../api/history';
import { useAuthStore } from '../../store/authStore';
import type { HistoryAttachment, HistoryContribution, HistoryEntry } from '../../types/history';
import type { HistoryDeletionRequest } from '../../types/history';
import { historyContributionStatusColor, historyContributionStatusText } from './historyState';
import { HistoryContent } from './HistoryContent';
import { DeletionReasonPreview } from './DeletionReasonPreview';
import { loadContributionPreview, type ContributionPreview } from './contributionPreview';
import { extractToc, type TocItem } from './toc';
import { formatDate } from '../../utils/datetime';
import './history-wiki.css';

const displayTitle = (title: string) => title.replace(/\s*v\d+$/i, '');
const contributionNote = (item: HistoryContribution) =>
  item.review_comment || item.change_note || (item.status === 'pending' ? '等待审核' : '');

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
          </List.Item>
        )}
      />
    </>
  );
}

export function HistoryWikiPage() {
  const { message, modal } = App.useApp();
  const navigate = useNavigate();
  const user = useAuthStore((state) => state.user);
  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin';
  const isAlumni = user?.role === 'alumni';
  const [entries, setEntries] = useState<HistoryEntry[]>([]);
  const [mine, setMine] = useState<HistoryContribution[]>([]);
  const [active, setActive] = useState<HistoryEntry | null>(null);
  const [deletingEntryID, setDeletingEntryID] = useState<number | null>(null);
  const [deletionRequests, setDeletionRequests] = useState<HistoryDeletionRequest[]>([]);
  const [requestEntry, setRequestEntry] = useState<HistoryEntry | null>(null);
  const [deletionReason, setDeletionReason] = useState('');
  const [requestingDeletion, setRequestingDeletion] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [searched, setSearched] = useState(false);
  const [mobileDirectoryOpen, setMobileDirectoryOpen] = useState(() => window.innerWidth <= 1180);
  const [activeTocID, setActiveTocID] = useState('');
  const [contributionPreview, setContributionPreview] = useState<ContributionPreview | null>(null);
  const selectedContribution = contributionPreview?.contribution ?? null;
  const [contributionLoading, setContributionLoading] = useState(false);
  const contributionRequest = useRef(0);
  const [selectedAttachments, setSelectedAttachments] = useState<HistoryAttachment[]>([]);
  const [attachmentsLoading, setAttachmentsLoading] = useState(false);
  const [preview, setPreview] = useState<{ url: string; name: string; image: boolean } | null>(
    null,
  );
  const [contentDetailOpen, setContentDetailOpen] = useState(false);
  const load = async (search = keyword) => {
    try {
      const [items, contributions, requests] = await Promise.all([
        historyApi.listEntries(search.trim() || undefined),
        isAlumni || isAdmin ? historyApi.listMine() : Promise.resolve([]),
        isAdmin ? historyApi.listMyDeletionRequests() : Promise.resolve([]),
      ]);
      setEntries(items);
      setDeletionRequests(requests);
      setMine(
        [...contributions].sort(
          (left, right) =>
            new Date(right.updated_at).getTime() - new Date(left.updated_at).getTime(),
        ),
      );
    } catch (error) {
      message.error(error instanceof Error ? error.message : '院史内容加载失败');
    }
  };
  useEffect(() => {
    setActive((current) => entries.find((item) => item.id === current?.id) ?? entries[0] ?? null);
  }, [entries]);
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
  const tocBaseLevel = toc.length ? Math.min(...toc.map((item) => item.level)) : 1;
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
    const request = ++contributionRequest.current;
    setContentDetailOpen(false);
    setContributionPreview(null);
    setContributionLoading(true);
    setSelectedAttachments([]);
    try {
      const current = await loadContributionPreview(contribution.id);
      if (request !== contributionRequest.current) return;
      setContributionPreview(current);
      setMine((items) =>
        items.map((item) => (item.id === current.contribution.id ? current.contribution : item)),
      );
    } catch (error) {
      if (request === contributionRequest.current)
        message.error(error instanceof Error ? error.message : '正文加载失败');
      return;
    } finally {
      if (request === contributionRequest.current) setContributionLoading(false);
    }
    setAttachmentsLoading(true);
    try {
      const attachments = await historyApi.listContributionAttachments(contribution.id);
      if (request === contributionRequest.current) setSelectedAttachments(attachments);
    } catch (error) {
      if (request !== contributionRequest.current) return;
      message.error(error instanceof Error ? error.message : '附件加载失败');
    } finally {
      if (request === contributionRequest.current) setAttachmentsLoading(false);
    }
  };
  const submitDeletionRequest = async () => {
    if (!requestEntry || !deletionReason.trim()) {
      message.warning('请填写删除原因');
      return;
    }
    setRequestingDeletion(true);
    try {
      const request = await historyApi.requestEntryDeletion(requestEntry.id, deletionReason.trim());
      setDeletionRequests((items) => [request, ...items]);
      setRequestEntry(null);
      setDeletionReason('');
      message.success('删除申请已提交，管理员同意后才会删除词条');
    } catch (error) {
      message.error(error instanceof Error ? error.message : '删除申请提交失败');
    } finally {
      setRequestingDeletion(false);
    }
  };
  const deleteEntry = async (entry: HistoryEntry) => {
    setDeletingEntryID(entry.id);
    try {
      await historyApi.deleteEntry(entry.id);
      setEntries((items) => items.filter((item) => item.id !== entry.id));
      message.success('词条已删除');
    } catch (error) {
      message.error(error instanceof Error ? error.message : '词条删除失败');
      throw error;
    } finally {
      setDeletingEntryID(null);
    }
  };
  const confirmDeleteEntry = (entry: HistoryEntry) => {
    modal.confirm({
      title: `删除词条“${displayTitle(entry.title)}”？`,
      content: '删除后词条将不再展示，历史投稿、版本和附件记录会保留。',
      okText: '删除词条',
      cancelText: '取消',
      okButtonProps: { danger: true },
      autoFocusButton: 'cancel',
      onOk: () => deleteEntry(entry),
    });
  };
  const deleteDraft = async (contribution: HistoryContribution) => {
    try {
      await historyApi.deleteDraft(contribution.id);
      setMine((current) => current.filter((item) => item.id !== contribution.id));
      if (selectedContribution?.id === contribution.id) setContributionPreview(null);
      message.success('草稿已删除');
    } catch (error) {
      message.error(error instanceof Error ? error.message : '草稿删除失败');
    }
  };
  const openContributionEditor = (contribution: HistoryContribution) => {
    setContributionPreview(null);
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
            placeholder="搜索正式词条的标题、简介或正文"
            allowClear
            onClear={() => {
              setSearched(false);
              void load('');
            }}
          />
          <Button onClick={search}>搜索</Button>
          {(isAlumni || isAdmin) && (
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
            <span>最近更新：{active ? formatDate(active.updated_at) : '—'}</span>
            <Space className="history-page__meta-actions" wrap>
              {user?.role === 'alumni' && active && (
                <Button onClick={() => openEditor('contribute')}>补充修改</Button>
              )}
              {isAdmin && active?.can_request_deletion && (
                <Button
                  danger
                  disabled={deletionRequests.some(
                    (item) => item.entry_id === active.id && item.status === 'pending',
                  )}
                  onClick={() => {
                    setRequestEntry(active);
                    setDeletionReason('');
                  }}
                >
                  {deletionRequests.some(
                    (item) => item.entry_id === active.id && item.status === 'pending',
                  )
                    ? '删除申请待审核'
                    : '申请删除词条'}
                </Button>
              )}
              {isAdmin && active && (
                <Space wrap>
                  <Button
                    type="primary"
                    icon={<EditOutlined />}
                    disabled={deletingEntryID !== null}
                    onClick={() => openEditor('admin-edit')}
                  >
                    编辑词条
                  </Button>
                  {active.can_delete && (
                    <Button
                      danger
                      aria-label="删除词条"
                      icon={<DeleteOutlined />}
                      loading={deletingEntryID === active.id}
                      disabled={deletingEntryID !== null && deletingEntryID !== active.id}
                      onClick={() => confirmDeleteEntry(active)}
                    >
                      删除词条
                    </Button>
                  )}
                </Space>
              )}
            </Space>
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
                    className={`${item.level > tocBaseLevel ? 'is-subsection' : ''} ${item.id === activeTocID ? 'active' : ''}`}
                    style={{ paddingLeft: 10 + (item.level - tocBaseLevel) * 10 }}
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
          {(isAlumni || isAdmin) && (
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
                        最后修改：{formatDate(item.updated_at)}
                      </span>
                    </div>
                  </List.Item>
                )}
              />
            </aside>
          )}
          {isAdmin && deletionRequests.length > 0 && (
            <aside className="history-page__mine">
              <strong>我的删除申请</strong>
              <List
                dataSource={deletionRequests}
                renderItem={(item) => (
                  <List.Item>
                    <div className="history-page__mine-item-content">
                      <Tag
                        color={
                          item.status === 'pending'
                            ? 'blue'
                            : item.status === 'approved'
                              ? 'green'
                              : 'red'
                        }
                      >
                        {
                          { pending: '待审核', approved: '已同意删除', rejected: '已驳回' }[
                            item.status
                          ]
                        }
                      </Tag>
                      <b>{displayTitle(item.title)}</b>
                      <DeletionReasonPreview reason={item.reason} />
                      {item.review_comment && <p>审核意见：{item.review_comment}</p>}
                      <span className="history-page__mine-item-time">
                        申请时间：{formatDate(item.created_at)}
                      </span>
                    </div>
                  </List.Item>
                )}
              />
            </aside>
          )}
        </div>
      </main>
      <Modal
        open={Boolean(requestEntry)}
        title="申请删除词条"
        okText="提交申请"
        cancelText="取消"
        confirmLoading={requestingDeletion}
        onOk={() => void submitDeletionRequest()}
        onCancel={() => {
          if (!requestingDeletion) setRequestEntry(null);
        }}
      >
        <p>词条：{requestEntry?.title}</p>
        <p>申请提交后词条仍会保留，管理员同意后才会删除。</p>
        <div className="history-page__deletion-reason">
          <Input.TextArea
            aria-label="删除原因"
            placeholder="请填写删除原因"
            value={deletionReason}
            maxLength={2000}
            rows={4}
            onChange={(event) => setDeletionReason(event.target.value)}
          />
          <span className="history-page__deletion-count" aria-live="polite">
            {deletionReason.length} / 2000
          </span>
        </div>
      </Modal>
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
        open={contributionLoading || Boolean(selectedContribution)}
        className="history-page__contribution-modal"
        title={<span className="history-page__contribution-title">投稿详情</span>}
        footer={null}
        onCancel={() => {
          ++contributionRequest.current;
          setContributionLoading(false);
          setContributionPreview(null);
          setContentDetailOpen(false);
        }}
        width={720}
      >
        {contributionLoading && <Spin />}
        {selectedContribution && (
          <>
            <dl className="history-page__contribution-details">
              <div className="history-page__contribution-row">
                <dt>标题</dt>
                <dd>{selectedContribution.title}</dd>
              </div>
              <div className="history-page__contribution-row">
                <dt>状态</dt>
                <dd>
                  <Tag color={historyContributionStatusColor[selectedContribution.status]}>
                    {historyContributionStatusText[selectedContribution.status]}
                  </Tag>
                </dd>
              </div>
              <div className="history-page__contribution-row">
                <dt>正文</dt>
                <dd>
                  <div className="history-page__contribution-preview">
                    <span>{contributionPreview?.content}</span>
                    <Button type="link" onClick={() => setContentDetailOpen(true)}>
                      查看详情
                    </Button>
                  </div>
                </dd>
              </div>
              <div className="history-page__contribution-row">
                <dt>资料来源</dt>
                <dd>{selectedContribution.source_note || '—'}</dd>
              </div>
              <div className="history-page__contribution-row">
                <dt>审核意见</dt>
                <dd>{selectedContribution.review_comment || '—'}</dd>
              </div>
            </dl>
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
        className="history-page__content-detail-modal"
        title="正文详情"
        footer={null}
        onCancel={() => setContentDetailOpen(false)}
        width={760}
      >
        <div className="history-page__contribution-content">
          <HistoryContent content={contributionPreview?.content ?? ''} />
        </div>
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
