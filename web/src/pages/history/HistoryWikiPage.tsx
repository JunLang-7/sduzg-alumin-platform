import { useEffect, useState } from 'react';
import {
  EditOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  PaperClipOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import { App, Button, Checkbox, Drawer, Empty, Form, Input, List, Space, Tag } from 'antd';
import { historyApi } from '../../api/history';
import { useAuthStore } from '../../store/authStore';
import type { HistoryContribution, HistoryEntry } from '../../types/history';
import { historyContributionStatusColor, historyContributionStatusText } from './historyState';
import './history-wiki.css';

const allowedTypes = ['image/jpeg', 'image/png', 'image/webp', 'application/pdf'];
type EditMode = 'create' | 'contribute' | 'admin-edit';
const displayTitle = (title: string) => title.replace(/\s*v\d+$/i, '');

export function HistoryWikiPage() {
  const { message } = App.useApp();
  const user = useAuthStore((state) => state.user);
  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin';
  const [entries, setEntries] = useState<HistoryEntry[]>([]);
  const [mine, setMine] = useState<HistoryContribution[]>([]);
  const [active, setActive] = useState<HistoryEntry | null>(null);
  const [keyword, setKeyword] = useState('');
  const [searched, setSearched] = useState(false);
  const [directoryOpen, setDirectoryOpen] = useState(true);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [mode, setMode] = useState<EditMode>('create');
  const [files, setFiles] = useState<File[]>([]);
  const [consent, setConsent] = useState(false);
  const [form] = Form.useForm();

  const load = async (search = keyword) => {
    try {
      const [entryItems, contributions] = await Promise.all([
        historyApi.listEntries(search.trim() || undefined),
        user?.role === 'alumni' ? historyApi.listMine() : Promise.resolve([]),
      ]);
      setEntries(entryItems);
      setMine(contributions);
      setActive(
        (current) => entryItems.find((item) => item.id === current?.id) ?? entryItems[0] ?? null,
      );
    } catch (error) {
      message.error(error instanceof Error ? error.message : '院史内容加载失败');
    }
  };
  useEffect(() => {
    void load(''); // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.role]);

  const openEditor = (nextMode: EditMode) => {
    if (nextMode !== 'create' && !active) return;
    setMode(nextMode);
    setFiles([]);
    setConsent(false);
    form.setFieldsValue(
      nextMode === 'create'
        ? { title: '', content: '', source_note: '', change_note: '' }
        : {
            title: displayTitle(active?.title ?? ''),
            content: active?.content,
            source_note: active?.source_note,
            change_note: '',
          },
    );
    setDrawerOpen(true);
  };
  const chooseFiles = (fileList: FileList | null) => {
    const selected = Array.from(fileList ?? []);
    if (selected.some((file) => !allowedTypes.includes(file.type)))
      return void message.error('仅支持 JPG、PNG、WebP、PDF 文件');
    if (selected.some((file) => file.size > 10 * 1024 * 1024))
      return void message.error('单个附件不能超过 10 MB');
    if (files.length + selected.length > 6) return void message.error('每次投稿最多上传 6 个附件');
    setFiles((current) => [...current, ...selected]);
  };
  const submit = async () => {
    try {
      const values = await form.validateFields();
      if (mode === 'admin-edit' && active) {
        await historyApi.updateEntry(active.id, values);
        message.success('词条已更新，历史版本已保留');
      } else {
        if (files.length && !consent)
          return void message.warning('上传图片或扫描件前，请确认来源和授权说明');
        const draft = await historyApi.createDraft({
          entry_id: mode === 'contribute' ? active?.id : undefined,
          ...values,
        });
        for (const file of files)
          await historyApi.uploadAttachment(draft.id, file, {
            description: file.name,
            source_note: values.source_note,
            rights_note: values.rights_note || '投稿人确认有权提交',
            consent_confirmed: consent,
          });
        await historyApi.submit(draft.id);
        message.success(mode === 'create' ? '新词条已提交审核' : '词条修改已提交审核');
      }
      setDrawerOpen(false);
      await load();
    } catch (error) {
      if (typeof error === 'object' && error !== null && 'errorFields' in error) return;
      message.error(error instanceof Error ? error.message : '提交失败，请稍后重试');
    }
  };
  const search = () => {
    setSearched(true);
    void load();
  };
  const drawerTitle =
    mode === 'create' ? '新建词条' : mode === 'admin-edit' ? '更改词条' : '补充词条内容';

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
      <Drawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        width={640}
        title={drawerTitle}
        footer={
          <Space>
            <Button onClick={() => setDrawerOpen(false)}>取消</Button>
            <Button type="primary" onClick={() => void submit()}>
              {mode === 'admin-edit' ? '保存更改' : '提交审核'}
            </Button>
          </Space>
        }
      >
        <Form form={form} layout="vertical">
          <Form.Item name="title" label="词条标题" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="content" label="正文" rules={[{ required: true }]}>
            <Input.TextArea rows={8} />
          </Form.Item>
          <Form.Item name="source_note" label="资料来源" rules={[{ required: true }]}>
            <Input.TextArea rows={3} placeholder="请注明资料出处" />
          </Form.Item>
          <Form.Item name="change_note" label={mode === 'create' ? '词条简介（可选）' : '修改说明'}>
            <Input />
          </Form.Item>
          {mode !== 'admin-edit' && (
            <>
              <Form.Item name="rights_note" label="附件授权说明">
                <Input.TextArea rows={2} />
              </Form.Item>
              <div className="history-page__upload">
                <PaperClipOutlined />
                <div>
                  <b>图片和扫描件</b>
                  <p>支持 JPG、PNG、WebP、PDF；单个不超过 10 MB；每次最多 6 个。</p>
                  <input
                    type="file"
                    multiple
                    accept="image/jpeg,image/png,image/webp,application/pdf"
                    onChange={(event) => {
                      chooseFiles(event.target.files);
                      event.currentTarget.value = '';
                    }}
                  />
                  {files.map((file) => (
                    <div key={`${file.name}-${file.size}`}>
                      {file.name}{' '}
                      <Button
                        type="link"
                        danger
                        onClick={() =>
                          setFiles((current) => current.filter((item) => item !== file))
                        }
                      >
                        移除
                      </Button>
                    </div>
                  ))}
                  <Checkbox
                    checked={consent}
                    onChange={(event) => setConsent(event.target.checked)}
                  >
                    我确认附件来源真实且有权提交
                  </Checkbox>
                </div>
              </div>
            </>
          )}
        </Form>
      </Drawer>
    </section>
  );
}
