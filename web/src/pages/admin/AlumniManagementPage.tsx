import { useEffect, useMemo, useRef, useState } from 'react';
import {
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  FileTextOutlined,
  PlusOutlined,
  SearchOutlined,
  UndoOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import {
  Button,
  Card,
  Dropdown,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  message,
} from 'antd';
import type { MenuProps } from 'antd';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import { useSearchParams } from 'react-router-dom';
import { alumniApi } from '../../api/alumni';
import { PageHeader } from '../../components/PageHeader';
import { StatusText } from '../../components/StatusText';
import type { AlumniImportResult, AlumniProfile, AlumniProfilePayload, AlumniQuery } from '../../types/alumni';
import { genderOptions, industryOptions, trainingModeOptions } from '../../utils/dictionaries';

const defaultPageSize = 20;

export function AlumniManagementPage() {
  const [searchForm] = Form.useForm<AlumniQuery>();
  const [modalForm] = Form.useForm<AlumniProfilePayload>();
  const [searchParams, setSearchParams] = useSearchParams();
  const urlKeyword = searchParams.get('keyword') || undefined;
  const [items, setItems] = useState<AlumniProfile[]>([]);
  const [total, setTotal] = useState(0);
  const [query, setQuery] = useState<AlumniQuery>({
    page: 1,
    page_size: defaultPageSize,
    keyword: urlKeyword,
  });
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState<AlumniProfile | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [importing, setImporting] = useState(false);
  const [importResult, setImportResult] = useState<AlumniImportResult | null>(null);
  const [importModalOpen, setImportModalOpen] = useState(false);
  const dataRequestIdRef = useRef(0);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const loadData = async (nextQuery: AlumniQuery) => {
    const requestId = dataRequestIdRef.current + 1;
    dataRequestIdRef.current = requestId;
    setLoading(true);
    try {
      const result = await alumniApi.list(nextQuery);
      if (requestId !== dataRequestIdRef.current) {
        return;
      }
      setItems(result.items || []);
      setTotal(result.total || 0);
    } catch (error) {
      if (requestId !== dataRequestIdRef.current) {
        return;
      }
      const err = error as Error;
      message.error(err.message || '校友数据加载失败');
    } finally {
      if (requestId === dataRequestIdRef.current) {
        setLoading(false);
      }
    }
  };

  useEffect(() => {
    void loadData(query);
  }, [query]);

  useEffect(() => {
    searchForm.setFieldsValue({ keyword: urlKeyword });
    setQuery((prev) => {
      if (prev.keyword === urlKeyword && prev.page === 1) {
        return prev;
      }

      return { ...prev, page: 1, keyword: urlKeyword };
    });
  }, [searchForm, urlKeyword]);

  const openCreateModal = () => {
    setEditing(null);
    modalForm.resetFields();
    setModalOpen(true);
  };

  const openEditModal = (record: AlumniProfile) => {
    setEditing(record);
    modalForm.setFieldsValue(record);
    setModalOpen(true);
  };

  const closeModal = () => {
    setModalOpen(false);
    setEditing(null);
    modalForm.resetFields();
  };

  const handleSave = async () => {
    const values = await modalForm.validateFields();
    setSaving(true);
    try {
      if (editing) {
        await alumniApi.update(editing.id, values);
        message.success('校友档案已更新');
      } else {
        await alumniApi.create(values);
        message.success('校友档案已新增');
      }
      closeModal();
      await loadData(query);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const handleRemove = async (record: AlumniProfile) => {
    try {
      await alumniApi.remove(record.id);
      message.success('校友档案已删除');
      await loadData(query);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '删除失败');
    }
  };

  const columns = useMemo<ColumnsType<AlumniProfile>>(
    () => [
      {
        title: '姓名',
        dataIndex: 'name',
        fixed: 'left',
        width: 120,
      },
      {
        title: '年级',
        dataIndex: 'grade',
        width: 110,
      },
      {
        title: '班级',
        dataIndex: 'class_name',
        width: 180,
      },
      {
        title: '届数',
        dataIndex: 'cohort',
        width: 110,
      },
      {
        title: '专业',
        dataIndex: 'major',
        width: 140,
      },
      {
        title: '行业',
        dataIndex: 'industry',
        width: 140,
      },
      {
        title: '工作单位',
        dataIndex: 'work_unit',
        width: 220,
        ellipsis: true,
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 100,
        render: (value: string) => <StatusText value={value} />,
      },
      {
        title: '操作',
        key: 'action',
        fixed: 'right',
        width: 160,
        render: (_, record) => (
          <Space size={4}>
            <Button type="link" icon={<EditOutlined />} onClick={() => openEditModal(record)}>
              编辑
            </Button>
            <Popconfirm
              title="删除校友档案"
              description="确认删除该校友档案？"
              onConfirm={() => handleRemove(record)}
            >
              <Button type="link" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [handleRemove, openEditModal],
  );

  const handleSearch = (values: AlumniQuery) => {
    const keyword = values.keyword?.trim();
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      if (keyword) {
        next.set('keyword', keyword);
      } else {
        next.delete('keyword');
      }
      return next;
    });
    setQuery({
      ...values,
      keyword,
      page: 1,
      page_size: query.page_size || defaultPageSize,
    });
  };

  const handleReset = () => {
    searchForm.resetFields();
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      next.delete('keyword');
      return next;
    });
    setQuery({ page: 1, page_size: defaultPageSize });
  };

  const handleTableChange = (pagination: TablePaginationConfig) => {
    setQuery((prev) => ({
      ...prev,
      page: pagination.current || 1,
      page_size: pagination.pageSize || defaultPageSize,
    }));
  };

  const handleExport = async (format?: string) => {
    try {
      const filters = searchForm.getFieldsValue();
      const blob = await alumniApi.exportData({ ...filters, format });
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      const ext = format === 'csv' ? 'csv' : 'xlsx';
      link.href = url;
      link.download = `alumni_export.${ext}`;
      link.click();
      window.URL.revokeObjectURL(url);
      message.success('导出成功');
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '导出失败');
    }
  };

  const handleImportClick = () => {
    fileInputRef.current?.click();
  };

  const handleDownloadTemplate = async () => {
    try {
      const blob = await alumniApi.downloadTemplate();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = 'alumni_import_template.xlsx';
      link.click();
      window.URL.revokeObjectURL(url);
      message.success('模板已下载');
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '模板下载失败');
    }
  };

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setImporting(true);
    try {
      const result = await alumniApi.importData(file);
      setImportResult(result);
      setImportModalOpen(true);
      await loadData(query);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '导入失败');
    } finally {
      setImporting(false);
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    }
  };

  const exportMenuItems: MenuProps['items'] = [
    { key: 'xlsx', label: '为 Excel (.xlsx)' },
    { key: 'csv', label: '为 CSV (.csv)' },
  ];

  return (
    <>
      <PageHeader
        title="校友管理"
        description="管理员维护 MPA 校友基础档案"
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreateModal}>
            新增校友
          </Button>
        }
      />
      <Card className="tool-card">
        <Form form={searchForm} layout="inline" onFinish={handleSearch} className="search-form">
          <Form.Item name="keyword">
            <Input allowClear placeholder="姓名、单位、导师" />
          </Form.Item>
          <Form.Item name="grade">
            <Input allowClear placeholder="年级" />
          </Form.Item>
          <Form.Item name="class_name">
            <Input allowClear placeholder="班级" />
          </Form.Item>
          <Form.Item name="cohort">
            <Input allowClear placeholder="届数" />
          </Form.Item>
          <Form.Item name="industry">
            <Select
              allowClear
              placeholder="行业"
              options={industryOptions.map((value) => ({ label: value, value }))}
            />
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" icon={<SearchOutlined />}>
              查询
            </Button>
            <Button icon={<UndoOutlined />} onClick={handleReset}>
              重置
            </Button>
            <Button icon={<UploadOutlined />} loading={importing} onClick={handleImportClick}>
              导入 Excel
            </Button>
            <Button icon={<FileTextOutlined />} onClick={handleDownloadTemplate}>
              导出模板
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              accept=".xlsx"
              style={{ display: 'none' }}
              onChange={handleFileChange}
            />
            <Dropdown
              menu={{
                items: exportMenuItems,
                onClick: ({ key }) => handleExport(key),
              }}
            >
              <Button icon={<DownloadOutlined />}>
                导出
              </Button>
            </Dropdown>
          </Space>
        </Form>
      </Card>
      <Card className="tool-card">
        <Table<AlumniProfile>
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={items}
          scroll={{ x: 1300 }}
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
        title={editing ? '编辑校友' : '新增校友'}
        open={modalOpen}
        onCancel={closeModal}
        onOk={handleSave}
        confirmLoading={saving}
        width={820}
        destroyOnClose
      >
        <Form form={modalForm} layout="vertical" className="modal-grid">
          <Form.Item
            label="姓名"
            name="name"
            rules={[{ required: true, message: '请输入姓名' }]}
          >
            <Input maxLength={100} />
          </Form.Item>
          <Form.Item
            label="年级"
            name="grade"
            rules={[{ required: true, message: '请输入年级' }]}
          >
            <Input maxLength={50} />
          </Form.Item>
          <Form.Item label="班级" name="class_name">
            <Input maxLength={100} />
          </Form.Item>
          <Form.Item label="届数" name="cohort">
            <Input maxLength={50} />
          </Form.Item>
          <Form.Item label="性别" name="gender">
            <Select
              allowClear
              options={genderOptions.map((value) => ({ label: value, value }))}
            />
          </Form.Item>
          <Form.Item label="手机号" name="mobile">
            <Input maxLength={30} />
          </Form.Item>
          <Form.Item label="邮箱" name="email">
            <Input maxLength={255} />
          </Form.Item>
          <Form.Item label="专业" name="major">
            <Input maxLength={100} />
          </Form.Item>
          <Form.Item label="培养方式" name="training_mode">
            <Select
              allowClear
              options={trainingModeOptions.map((value) => ({ label: value, value }))}
            />
          </Form.Item>
          <Form.Item label="辅导员" name="counselor">
            <Input maxLength={100} />
          </Form.Item>
          <Form.Item label="导师" name="mentor">
            <Input maxLength={100} />
          </Form.Item>
          <Form.Item label="行业" name="industry">
            <Select
              allowClear
              options={industryOptions.map((value) => ({ label: value, value }))}
            />
          </Form.Item>
          <Form.Item label="工作单位" name="work_unit">
            <Input maxLength={255} />
          </Form.Item>
          <Form.Item label="职务" name="position">
            <Input maxLength={100} />
          </Form.Item>
          <Form.Item label="通讯地址" name="mailing_address" className="modal-grid-wide">
            <Input.TextArea rows={3} maxLength={255} showCount />
          </Form.Item>
          <Form.Item label="管理员备注" name="remark" className="modal-grid-wide">
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        title="导入结果"
        open={importModalOpen}
        onCancel={() => setImportModalOpen(false)}
        footer={
          <Button type="primary" onClick={() => setImportModalOpen(false)}>
            确定
          </Button>
        }
        destroyOnClose
      >
        {importResult && (
          <div>
            <p>
              共解析 <strong>{importResult.total}</strong> 条记录，成功导入{' '}
              <strong>{importResult.success}</strong> 条
            </p>
            {importResult.errors.length > 0 && (
              <Table
                dataSource={importResult.errors}
                rowKey="row"
                size="small"
                pagination={false}
                columns={[
                  { title: '行号', dataIndex: 'row', width: 70 },
                  { title: '姓名', dataIndex: 'name', width: 100 },
                  { title: '错误原因', dataIndex: 'message' },
                ]}
                style={{ marginTop: 12 }}
              />
            )}
            {importResult.errors.length === 0 && (
              <p style={{ color: '#52c41a', marginTop: 8 }}>全部导入成功，无错误记录</p>
            )}
          </div>
        )}
      </Modal>
    </>
  );
}
