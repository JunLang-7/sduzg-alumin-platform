import { useEffect, useMemo, useState } from 'react';
import { EyeOutlined, SearchOutlined, UndoOutlined } from '@ant-design/icons';
import { Button, Card, Form, Input, Select, Space, Table, message } from 'antd';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { alumniApi } from '../../api/alumni';
import { PageHeader } from '../../components/PageHeader';
import type { AlumniProfile, AlumniQuery } from '../../types/alumni';
import { industryOptions, trainingModeOptions } from '../../utils/dictionaries';

const defaultPageSize = 20;

export function AlumniListPage() {
  const [form] = Form.useForm<AlumniQuery>();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const urlKeyword = searchParams.get('keyword') || undefined;
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<AlumniProfile[]>([]);
  const [query, setQuery] = useState<AlumniQuery>({
    page: 1,
    page_size: defaultPageSize,
    keyword: urlKeyword,
  });
  const [total, setTotal] = useState(0);

  const loadData = async (nextQuery: AlumniQuery) => {
    setLoading(true);
    try {
      const result = await alumniApi.list(nextQuery);
      setItems(result.items || []);
      setTotal(result.total || 0);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '校友列表加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadData(query);
  }, [query]);

  useEffect(() => {
    form.setFieldsValue({ keyword: urlKeyword });
    setQuery((prev) => ({ ...prev, page: 1, keyword: urlKeyword }));
  }, [form, urlKeyword]);

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
        title: '培养方式',
        dataIndex: 'training_mode',
        width: 120,
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
        title: '职务',
        dataIndex: 'position',
        width: 140,
      },
      {
        title: '操作',
        key: 'action',
        fixed: 'right',
        width: 96,
        render: (_, record) => (
          <Button
            type="link"
            icon={<EyeOutlined />}
            onClick={() => navigate(`/alumni/${record.id}`)}
          >
            查看
          </Button>
        ),
      },
    ],
    [navigate],
  );

  const handleSearch = (values: AlumniQuery) => {
    setQuery({ ...values, page: 1, page_size: query.page_size || defaultPageSize });
  };

  const handleReset = () => {
    form.resetFields();
    setQuery({ page: 1, page_size: defaultPageSize });
  };

  const handleTableChange = (pagination: TablePaginationConfig) => {
    setQuery((prev) => ({
      ...prev,
      page: pagination.current || 1,
      page_size: pagination.pageSize || defaultPageSize,
    }));
  };

  return (
    <>
      <PageHeader title="校友列表" description="按基础信息、学习经历和职业信息检索校友档案" />
      <Card className="tool-card">
        <Form form={form} layout="inline" onFinish={handleSearch} className="search-form">
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
          <Form.Item name="major">
            <Input allowClear placeholder="专业" />
          </Form.Item>
          <Form.Item name="training_mode">
            <Select
              allowClear
              placeholder="培养方式"
              options={trainingModeOptions.map((value) => ({ label: value, value }))}
            />
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
          </Space>
        </Form>
      </Card>
      <Card className="tool-card">
        <Table<AlumniProfile>
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={items}
          scroll={{ x: 1450 }}
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
    </>
  );
}
