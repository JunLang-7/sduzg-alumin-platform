import { useEffect, useMemo, useState } from 'react';
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons';
import { Button, Card, Checkbox, Form, Input, Modal, Popconfirm, Select, Space, Table, Tag, message } from 'antd';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import { adminsApi } from '../../api/admins';
import { PageHeader } from '../../components/PageHeader';
import { StatusText } from '../../components/StatusText';
import type { AdminQuery, AdminUser, CreateAdminPayload, UpdateAdminAccessPayload } from '../../types/admin';
import type { DataDomain } from '../../types/auth';
import { permissionCodes } from '../../utils/access';

const defaultPageSize = 20;
const permissionOptions = [
  { label: '可查看敏感数据', value: permissionCodes.sensitiveRead },
  { label: '可管理档案文件', value: permissionCodes.filesManage },
];

export function AdminUsersPage() {
  const [form] = Form.useForm<CreateAdminPayload>();
  const [accessForm] = Form.useForm<UpdateAdminAccessPayload>();
  const [items, setItems] = useState<AdminUser[]>([]);
  const [domains, setDomains] = useState<DataDomain[]>([]);
  const [query, setQuery] = useState<AdminQuery>({ page: 1, page_size: defaultPageSize });
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<AdminUser | null>(null);

  const loadData = async (nextQuery: AdminQuery) => {
    setLoading(true);
    try {
      const result = await adminsApi.list(nextQuery);
      setItems(result.items || []);
      setTotal(result.total || 0);
    } catch (error) {
      message.error((error as Error).message || '管理员列表加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadData(query); }, [query]);
  useEffect(() => {
    adminsApi.listDataDomains().then(setDomains).catch((error: Error) => message.error(error.message || '可管理范围加载失败'));
  }, []);

  const closeCreateModal = () => { form.resetFields(); setModalOpen(false); };
  const handleCreate = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      await adminsApi.create(values);
      message.success('管理员已创建');
      closeCreateModal();
      await loadData(query);
    } catch (error) {
      message.error((error as Error).message || '创建失败');
    } finally { setSaving(false); }
  };

  const openAccessModal = async (record: AdminUser) => {
    setEditing(record);
    accessForm.setFieldsValue({ domain_ids: record.domains.map((domain) => domain.id), permissions: record.permissions });
    try {
      const detail = await adminsApi.detail(record.id);
      accessForm.setFieldsValue({ domain_ids: detail.domains.map((domain) => domain.id), permissions: detail.permissions });
    } catch (error) { message.error((error as Error).message || '管理员授权加载失败'); }
  };
  const closeAccessModal = () => { accessForm.resetFields(); setEditing(null); };
  const saveAccess = async () => {
    if (!editing) return;
    const values = await accessForm.validateFields();
    setSaving(true);
    try {
      await adminsApi.replaceAccess(editing.id, values);
      message.success('管理员授权已更新');
      closeAccessModal();
      await loadData(query);
    } catch (error) { message.error((error as Error).message || '保存授权失败');
    } finally { setSaving(false); }
  };
  const handleRemove = async (record: AdminUser) => {
    try {
      await adminsApi.remove(record.id);
      message.success('管理员已删除');
      if (items.length === 1 && (query.page || 1) > 1) {
        setQuery((current) => ({ ...current, page: (current.page || 1) - 1 }));
        return;
      }
      await loadData(query);
    }
    catch (error) { message.error((error as Error).message || '删除失败'); }
  };

  const columns = useMemo<ColumnsType<AdminUser>>(() => [
    { title: '账号', dataIndex: 'account' },
    { title: '姓名', dataIndex: 'real_name' },
    { title: '可管理范围', dataIndex: 'domains', render: (value: DataDomain[]) => value.map((domain) => <Tag key={domain.id}>{domain.name}</Tag>) },
    { title: '敏感数据', dataIndex: 'permissions', render: (value: string[]) => value.includes(permissionCodes.sensitiveRead) ? <Tag color="green">可查看</Tag> : <Tag>不可查看</Tag> },
    { title: '档案文件', dataIndex: 'permissions', render: (value: string[]) => value.includes(permissionCodes.filesManage) ? <Tag color="blue">可管理</Tag> : <Tag>不可管理</Tag> },
    { title: '状态', dataIndex: 'status', width: 100, render: (value: string) => <StatusText value={value} /> },
    {
      title: '操作',
      key: 'action',
      width: 180,
      render: (_, record) => record.role === 'super_admin' ? (
        <Tag color="gold">系统超级管理员</Tag>
      ) : (
        <Space size={4}>
          <Button type="link" icon={<EditOutlined />} onClick={() => void openAccessModal(record)}>编辑授权</Button>
          <Popconfirm title="删除管理员" description="确认删除该管理员账号？" onConfirm={() => handleRemove(record)}><Button type="link" danger icon={<DeleteOutlined />}>删除</Button></Popconfirm>
        </Space>
      ),
    },
  ], [items.length, query]);

  return <>
    <PageHeader title="管理员管理" description="配置管理员可管理范围与功能权限" extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>创建管理员</Button>} />
    <Card className="tool-card"><Table rowKey="id" loading={loading} columns={columns} dataSource={items} pagination={{ current: query.page, pageSize: query.page_size, total, showSizeChanger: true, showTotal: (value) => `共 ${value} 条` }} onChange={(pagination: TablePaginationConfig) => setQuery({ page: pagination.current || 1, page_size: pagination.pageSize || defaultPageSize })} /></Card>
    <Modal title="创建管理员" open={modalOpen} onCancel={closeCreateModal} onOk={() => void handleCreate()} confirmLoading={saving} destroyOnClose>
      <Form form={form} layout="vertical" initialValues={{ domain_ids: [], permissions: [] }}>
        <Form.Item label="账号" name="account" rules={[{ required: true, message: '请输入账号' }]}><Input maxLength={100} /></Form.Item>
        <Form.Item label="姓名" name="real_name" rules={[{ required: true, message: '请输入姓名' }]}><Input maxLength={100} /></Form.Item>
        <Form.Item label="手机号" name="mobile"><Input maxLength={30} /></Form.Item>
        <Form.Item label="初始密码" name="password" rules={[{ required: true, message: '请输入初始密码' }, { min: 8, message: '密码至少 8 位' }]}><Input.Password autoComplete="new-password" /></Form.Item>
        <AccessFields domains={domains} />
      </Form>
    </Modal>
    <Modal title={`编辑授权${editing ? ` · ${editing.account}` : ''}`} open={Boolean(editing)} onCancel={closeAccessModal} onOk={() => void saveAccess()} confirmLoading={saving} destroyOnClose>
      <Form form={accessForm} layout="vertical"><AccessFields domains={domains} /></Form>
    </Modal>
  </>;
}

function AccessFields({ domains }: { domains: DataDomain[] }) {
  return <>
    <Form.Item label="可管理范围" name="domain_ids" rules={[{ required: true, message: '请至少选择一项可管理范围' }]}>
      <Select mode="multiple" placeholder="选择可管理的培养类别" options={domains.map((domain) => ({ label: domain.name, value: domain.id }))} />
    </Form.Item>
    <Form.Item label="功能权限" name="permissions"><Checkbox.Group options={permissionOptions} /></Form.Item>
  </>;
}
