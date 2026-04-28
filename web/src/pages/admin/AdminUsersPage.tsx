import { useEffect, useMemo, useState } from 'react';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { Button, Card, Form, Input, Modal, Popconfirm, Table, message } from 'antd';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import { adminsApi } from '../../api/admins';
import { PageHeader } from '../../components/PageHeader';
import { StatusText } from '../../components/StatusText';
import type { AdminQuery, AdminUser, CreateAdminPayload } from '../../types/admin';

const defaultPageSize = 20;

export function AdminUsersPage() {
  const [form] = Form.useForm<CreateAdminPayload>();
  const [items, setItems] = useState<AdminUser[]>([]);
  const [query, setQuery] = useState<AdminQuery>({ page: 1, page_size: defaultPageSize });
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);

  const loadData = async (nextQuery: AdminQuery) => {
    setLoading(true);
    try {
      const result = await adminsApi.list(nextQuery);
      setItems(result.items || []);
      setTotal(result.total || 0);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '管理员列表加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadData(query);
  }, [query]);

  const closeModal = () => {
    form.resetFields();
    setModalOpen(false);
  };

  const handleCreate = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      await adminsApi.create(values);
      message.success('管理员已创建');
      closeModal();
      await loadData(query);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '创建失败');
    } finally {
      setSaving(false);
    }
  };

  const handleRemove = async (record: AdminUser) => {
    try {
      await adminsApi.remove(record.id);
      message.success('管理员已删除');
      await loadData(query);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '删除失败');
    }
  };

  const columns = useMemo<ColumnsType<AdminUser>>(
    () => [
      {
        title: '账号',
        dataIndex: 'account',
      },
      {
        title: '姓名',
        dataIndex: 'real_name',
      },
      {
        title: '手机号',
        dataIndex: 'mobile',
      },
      {
        title: '角色',
        dataIndex: 'role',
        width: 130,
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 120,
        render: (value: string) => <StatusText value={value} />,
      },
      {
        title: '最近登录',
        dataIndex: 'last_login_at',
        width: 180,
        render: (value?: string) => value || '-',
      },
      {
        title: '操作',
        key: 'action',
        width: 110,
        render: (_, record) => (
          <Popconfirm
            title="删除管理员"
            description="确认删除该管理员账号？"
            onConfirm={() => handleRemove(record)}
          >
            <Button type="link" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        ),
      },
    ],
    [handleRemove],
  );

  const handleTableChange = (pagination: TablePaginationConfig) => {
    setQuery({
      page: pagination.current || 1,
      page_size: pagination.pageSize || defaultPageSize,
    });
  };

  return (
    <>
      <PageHeader
        title="管理员管理"
        description="超级管理员创建和删除运营管理员账号"
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
            创建管理员
          </Button>
        }
      />
      <Card className="tool-card">
        <Table<AdminUser>
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={items}
          pagination={{
            current: query.page,
            pageSize: query.page_size,
            total,
            showSizeChanger: true,
            showTotal: (value) => `共 ${value} 条`,
          }}
          onChange={handleTableChange}
        />
      </Card>
      <Modal
        title="创建管理员"
        open={modalOpen}
        onCancel={closeModal}
        onOk={handleCreate}
        confirmLoading={saving}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item
            label="账号"
            name="account"
            rules={[{ required: true, message: '请输入账号' }]}
          >
            <Input maxLength={100} />
          </Form.Item>
          <Form.Item
            label="姓名"
            name="real_name"
            rules={[{ required: true, message: '请输入姓名' }]}
          >
            <Input maxLength={100} />
          </Form.Item>
          <Form.Item label="手机号" name="mobile">
            <Input maxLength={30} />
          </Form.Item>
          <Form.Item
            label="初始密码"
            name="password"
            rules={[
              { required: true, message: '请输入初始密码' },
              { min: 8, message: '密码至少 8 位' },
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
