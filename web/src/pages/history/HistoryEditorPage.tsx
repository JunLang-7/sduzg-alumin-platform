import { ArrowLeftOutlined, PaperClipOutlined } from '@ant-design/icons';
import { App, Button, Checkbox, Empty, Form, Input, Space } from 'antd';
import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { historyApi } from '../../api/history';
import { useAuthStore } from '../../store/authStore';
import type { HistoryEntry } from '../../types/history';
import './history-editor.css';

const allowedTypes = ['image/jpeg', 'image/png', 'image/webp', 'application/pdf'];
type EditMode = 'create' | 'contribute' | 'admin-edit';
const displayTitle = (title: string) => title.replace(/\s*v\d+$/i, '');

export function HistoryEditorPage() {
  const { message } = App.useApp();
  const user = useAuthStore((state) => state.user);
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const mode = (params.get('mode') || 'create') as EditMode;
  const entryID = Number(params.get('entryId'));
  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin';
  const canEdit =
    (mode === 'create' && user?.role === 'alumni') ||
    (mode === 'contribute' && user?.role === 'alumni' && Number.isInteger(entryID)) ||
    (mode === 'admin-edit' && isAdmin && Number.isInteger(entryID));
  const [entry, setEntry] = useState<HistoryEntry | null>(null);
  const [loading, setLoading] = useState(mode !== 'create');
  const [files, setFiles] = useState<File[]>([]);
  const [consent, setConsent] = useState(false);
  const [form] = Form.useForm();

  useEffect(() => {
    if (!canEdit) return;
    if (mode === 'create') {
      form.setFieldsValue({ title: '', content: '', source_note: '', change_note: '' });
      return;
    }
    void historyApi
      .listEntries()
      .then((items) => {
        const current = items.find((item) => item.id === entryID) ?? null;
        setEntry(current);
        form.setFieldsValue(
          current
            ? {
                title: displayTitle(current.title),
                content: current.content,
                source_note: current.source_note,
                change_note: '',
              }
            : {},
        );
      })
      .catch((error) => message.error(error instanceof Error ? error.message : '词条加载失败'))
      .finally(() => setLoading(false));
  }, [canEdit, entryID, form, message, mode]);

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
      if (mode === 'admin-edit' && entry) {
        await historyApi.updateEntry(entry.id, values);
        message.success('词条已更新，历史版本已保留');
      } else {
        if (files.length && !consent)
          return void message.warning('上传图片或扫描件前，请确认来源和授权说明');
        const draft = await historyApi.createDraft({
          entry_id: mode === 'contribute' ? entry?.id : undefined,
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
      navigate('/history');
    } catch (error) {
      if (typeof error === 'object' && error !== null && 'errorFields' in error) return;
      message.error(error instanceof Error ? error.message : '提交失败，请稍后重试');
    }
  };
  const title =
    mode === 'create' ? '新建词条' : mode === 'admin-edit' ? '更改词条' : '补充词条内容';
  if (!canEdit)
    return (
      <section className="history-editor">
        <Empty description="无权编辑此词条" />
        <Button onClick={() => navigate('/history')}>返回院史共编</Button>
      </section>
    );

  return (
    <section className="history-editor">
      <header className="history-editor__header">
        <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/history')}>
          返回院史共编
        </Button>
        <h1>{title}</h1>
      </header>
      {loading ? (
        <Empty description="正在加载词条" />
      ) : (
        <div className="history-editor__card">
          <Form form={form} layout="vertical">
            <Form.Item name="title" label="词条标题" rules={[{ required: true }]}>
              <Input />
            </Form.Item>
            <Form.Item name="content" label="正文" rules={[{ required: true }]}>
              <Input.TextArea rows={12} />
            </Form.Item>
            <Form.Item name="source_note" label="资料来源" rules={[{ required: true }]}>
              <Input.TextArea rows={3} placeholder="请注明资料出处" />
            </Form.Item>
            <Form.Item
              name="change_note"
              label={mode === 'create' ? '词条简介（可选）' : '修改说明'}
            >
              <Input />
            </Form.Item>
            {mode !== 'admin-edit' && (
              <>
                <Form.Item name="rights_note" label="附件授权说明">
                  <Input.TextArea rows={2} />
                </Form.Item>
                <div className="history-editor__upload">
                  <PaperClipOutlined />
                  <div>
                    <b>图片和扫描件</b>
                    <p>支持 JPG、PNG、WebP、PDF；单个不超过 10 MB；每次最多 6 个</p>
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
          <footer>
            <Space>
              <Button onClick={() => navigate('/history')}>取消</Button>
              <Button type="primary" onClick={() => void submit()}>
                {mode === 'admin-edit' ? '保存更改' : '提交审核'}
              </Button>
            </Space>
          </footer>
        </div>
      )}
    </section>
  );
}
