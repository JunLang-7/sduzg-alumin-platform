import { ArrowLeftOutlined, PaperClipOutlined } from '@ant-design/icons';
import { App, Button, Checkbox, Empty, Form, Input, Select, Space } from 'antd';
import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { historyApi } from '../../api/history';
import { useAuthStore } from '../../store/authStore';
import type { HistoryContribution, HistoryEntry } from '../../types/history';
import './history-editor.css';

const allowedTypes = ['image/jpeg', 'image/png', 'image/webp', 'application/pdf'];
type EditMode = 'create' | 'contribute' | 'admin-edit' | 'contribution-edit';
type SubmissionAction = 'save' | 'submit';
const displayTitle = (title: string) => title.replace(/\s*v\d+$/i, '');

function SelectedFilePreview({ file }: { file: File }) {
  const [url, setURL] = useState('');

  useEffect(() => {
    const objectURL = URL.createObjectURL(file);
    setURL(objectURL);
    return () => URL.revokeObjectURL(objectURL);
  }, [file]);

  if (!url) return null;
  if (file.type.startsWith('image/')) {
    return <img className="history-editor__preview-image" src={url} alt={file.name} />;
  }
  return (
    <Button type="link" onClick={() => window.open(url, '_blank', 'noopener,noreferrer')}>
      预览文件
    </Button>
  );
}

export function HistoryEditorPage() {
  const { message } = App.useApp();
  const user = useAuthStore((state) => state.user);
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const mode = (params.get('mode') || 'create') as EditMode;
  const entryID = Number(params.get('entryId'));
  const contributionID = Number(params.get('contributionId'));
  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin';
  const canEdit =
    (mode === 'create' && (user?.role === 'alumni' || isAdmin)) ||
    (mode === 'contribute' && user?.role === 'alumni' && Number.isInteger(entryID)) ||
    (mode === 'admin-edit' && isAdmin && Number.isInteger(entryID)) ||
    (mode === 'contribution-edit' &&
      (user?.role === 'alumni' || isAdmin) &&
      Number.isInteger(contributionID));
  const [entry, setEntry] = useState<HistoryEntry | null>(null);
  const [contribution, setContribution] = useState<HistoryContribution | null>(null);
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
    if (mode === 'contribution-edit') {
      void historyApi
        .getContribution(contributionID)
        .then((current) => {
          setContribution(current);
          form.setFieldsValue({
            title: current.title,
            content: current.content,
            source_note: current.source_note,
            change_note: current.change_note,
          });
        })
        .catch((error) => message.error(error instanceof Error ? error.message : '投稿加载失败'))
        .finally(() => setLoading(false));
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
  }, [canEdit, contributionID, entryID, form, message, mode]);

  const chooseFiles = (fileList: FileList | null) => {
    const selected = Array.from(fileList ?? []);
    if (selected.some((file) => !allowedTypes.includes(file.type)))
      return void message.error('仅支持 JPG、PNG、WebP、PDF 文件');
    if (selected.some((file) => file.size > 10 * 1024 * 1024))
      return void message.error('单个附件不能超过 10 MB');
    if (files.length + selected.length > 6) return void message.error('每次投稿最多上传 6 个附件');
    setFiles((current) => [...current, ...selected]);
  };
  const saveContribution = async (action: SubmissionAction) => {
    try {
      const values = await form.validateFields();
      if (mode === 'admin-edit' && entry) {
        await historyApi.updateEntry(entry.id, values);
        message.success('词条已更新，历史版本已保留');
      } else {
        if (files.length && !consent)
          return void message.warning('上传图片或扫描件前，请确认来源和授权说明');
        const current =
          mode === 'contribution-edit' && contribution
            ? await historyApi.updateContribution(contribution.id, values)
            : await historyApi.createDraft({
                ...values,
                entry_id: mode === 'contribute' ? entry?.id : undefined,
                data_domain_id:
                  isAdmin && values.data_domain_id ? Number(values.data_domain_id) : undefined,
              });
        for (const file of files)
          await historyApi.uploadAttachment(current.id, file, {
            description: file.name,
            source_note: values.source_note,
            rights_note: values.rights_note || '投稿人确认有权提交',
            consent_confirmed: consent,
          });
        if (action === 'submit' && current.status !== 'pending')
          await historyApi.submit(current.id);
        if (action === 'save') {
          message.success(mode === 'create' ? '草稿已保存' : '投稿修改已保存');
        } else if (current.status === 'pending') {
          message.success('投稿修改已保存，审核将以最新内容为准');
        } else {
          message.success(mode === 'create' ? '新词条已提交审核' : '词条修改已提交审核');
        }
      }
      navigate('/history');
    } catch (error) {
      if (typeof error === 'object' && error !== null && 'errorFields' in error) return;
      message.error(error instanceof Error ? error.message : '提交失败，请稍后重试');
    }
  };
  const title =
    mode === 'create'
      ? '新建词条'
      : mode === 'admin-edit'
        ? '更改词条'
        : mode === 'contribution-edit'
          ? '编辑投稿'
          : '补充词条内容';
  const primaryAction: SubmissionAction = contribution?.status === 'pending' ? 'save' : 'submit';
  const primaryText =
    mode === 'admin-edit'
      ? '保存更改'
      : primaryAction === 'save'
        ? '保存修改'
        : contribution?.status === 'returned'
          ? '重新提交审核'
          : '提交审核';
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
                {isAdmin && (
                  <Form.Item name="data_domain_id" label="所属数据域" rules={[{ required: true }]}>
                    <Select
                      placeholder="请选择词条所属数据域"
                      options={(user?.domains ?? []).map((domain) => ({
                        label: domain.name,
                        value: domain.id,
                      }))}
                    />
                  </Form.Item>
                )}
                <Form.Item name="rights_note" label="附件授权说明">
                  <Input.TextArea rows={2} />
                </Form.Item>
                <div className="history-editor__upload">
                  <PaperClipOutlined />
                  <div>
                    <div className="history-editor__upload-title">图片和扫描件</div>
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
                      <div className="history-editor__file" key={`${file.name}-${file.size}`}>
                        <SelectedFilePreview file={file} />
                        <span>{file.name}</span>{' '}
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
              {mode === 'create' && (
                <Button onClick={() => void saveContribution('save')}>存为草稿</Button>
              )}
              {mode === 'contribution-edit' && contribution?.status !== 'pending' && (
                <Button onClick={() => void saveContribution('save')}>保存草稿</Button>
              )}
              <Button type="primary" onClick={() => void saveContribution(primaryAction)}>
                {primaryText}
              </Button>
            </Space>
          </footer>
        </div>
      )}
    </section>
  );
}
