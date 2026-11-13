import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  EyeOutlined,
  FileAddOutlined,
  FileTextOutlined,
  PlusOutlined,
  SearchOutlined,
  UndoOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Dropdown,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Table,
  Tabs,
  Tag,
  Upload,
  message,
} from 'antd';
import type { MenuProps } from 'antd';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { alumniApi } from '../../api/alumni';
import { PageHeader } from '../../components/PageHeader';
import { useAuthStore } from '../../store/authStore';
import type { DataDomain } from '../../types/auth';
import type {
  AlumniEnrollment,
  AlumniEnrollmentFileGroup,
  AlumniEnrollmentInput,
  AlumniFileItem,
  AlumniFileType,
  AlumniImportResult,
  AlumniListItem,
  AlumniProfile,
  AlumniProfilePayload,
  AlumniQuery,
  TrainingLevel,
} from '../../types/alumni';
import { EXPLICIT_TRAINING_LEVELS, TRAINING_LEVEL_LABELS } from '../../types/alumni';
import { canManageFiles, canReadSensitive } from '../../utils/access';
import { genderOptions, industryOptions, trainingModeOptions } from '../../utils/dictionaries';

const defaultPageSize = 20;
const emptyDomains: DataDomain[] = [];

const FILE_TYPE_LABELS: Record<AlumniFileType, string> = {
  degree_archive: '学位档案',
  academic_record: '学籍档案',
};

type FieldErrors = Record<string, string>;

interface DraftEnrollment {
  key: string;
  id?: number;
  version?: number;
  data_domain_id?: number;
  training_level: TrainingLevel | '';
  grade: string;
  class_name?: string;
  cohort?: string;
  counselor?: string;
  mentor?: string;
  major?: string;
  training_mode?: string;
  remark?: string;
  files: AlumniFileItem[];
  removedFileIds: number[];
  uploadedDraftIds: number[];
}

function levelLabel(level?: string): string {
  if (level && level in TRAINING_LEVEL_LABELS) {
    return TRAINING_LEVEL_LABELS[level as TrainingLevel];
  }
  return '待核验';
}

function defaultLevelForDomain(domain?: DataDomain): TrainingLevel {
  if (domain?.code === 'undergraduate') return 'undergraduate';
  return 'master';
}

function enrollmentSummary(enrollment: {
  training_level: string;
  grade: string;
  data_domain_name?: string;
}): string {
  return `${enrollment.grade ? `${enrollment.grade}级` : '年级未填'} ${levelLabel(enrollment.training_level)}${
    enrollment.data_domain_name ? ` · ${enrollment.data_domain_name}` : ''
  }`;
}

let draftCounter = 0;
function newDraftKey(): string {
  draftCounter += 1;
  return `new-${Date.now()}-${draftCounter}`;
}

export function AlumniManagementPage() {
  const user = useAuthStore((state) => state.user);
  const domains = user?.domains ?? emptyDomains;
  const sensitiveReadable = canReadSensitive(user);
  const filesManageable = canManageFiles(user);
  const [searchForm] = Form.useForm<AlumniQuery>();
  const [personalForm] = Form.useForm();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const urlKeyword = searchParams.get('keyword') || undefined;
  const [items, setItems] = useState<AlumniListItem[]>([]);
  const [total, setTotal] = useState(0);
  const [query, setQuery] = useState<AlumniQuery>({
    page: 1,
    page_size: defaultPageSize,
    keyword: urlKeyword,
  });
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [modalLoading, setModalLoading] = useState(false);
  const [editing, setEditing] = useState<AlumniProfile | null>(null);
  const [drafts, setDrafts] = useState<DraftEnrollment[]>([]);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [activeTab, setActiveTab] = useState('personal');
  const [uploadingKey, setUploadingKey] = useState<string | null>(null);
  const [importing, setImporting] = useState(false);
  const [importResult, setImportResult] = useState<AlumniImportResult | null>(null);
  const [importModalOpen, setImportModalOpen] = useState(false);
  const [importDomainID, setImportDomainID] = useState<number>();
  const dataRequestIdRef = useRef(0);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const loadData = useCallback(async (nextQuery: AlumniQuery) => {
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
  }, []);

  useEffect(() => {
    void loadData(query);
  }, [loadData, query]);

  useEffect(() => {
    searchForm.setFieldsValue({ keyword: urlKeyword });
    setQuery((prev) => {
      if (prev.keyword === urlKeyword && prev.page === 1) {
        return prev;
      }
      return { ...prev, page: 1, keyword: urlKeyword };
    });
  }, [searchForm, urlKeyword]);

  const resetEditor = useCallback(() => {
    setEditing(null);
    setDrafts([]);
    setFieldErrors({});
    setActiveTab('personal');
    setUploadingKey(null);
    personalForm.resetFields();
  }, [personalForm]);

  const buildDraftsFromProfile = useCallback(
    (profile: AlumniProfile, fileGroups: AlumniEnrollmentFileGroup[]): DraftEnrollment[] => {
      const filesByEnrollment = new Map<number, AlumniFileItem[]>();
      fileGroups.forEach((group) => {
        filesByEnrollment.set(group.enrollment_id, [
          ...group.degree_archive,
          ...group.academic_record,
        ]);
      });
      return profile.enrollments.map((enrollment: AlumniEnrollment) => ({
        key: `existing-${enrollment.id}`,
        id: enrollment.id,
        version: enrollment.version,
        data_domain_id: enrollment.data_domain_id,
        training_level: enrollment.training_level,
        grade: enrollment.grade,
        class_name: enrollment.class_name,
        cohort: enrollment.cohort,
        counselor: enrollment.counselor,
        mentor: enrollment.mentor,
        major: enrollment.major,
        training_mode: enrollment.training_mode,
        remark: enrollment.remark,
        files: filesByEnrollment.get(enrollment.id) ?? [],
        removedFileIds: [],
        uploadedDraftIds: [],
      }));
    },
    [],
  );

  const openCreateModal = () => {
    resetEditor();
    const defaultDomain = domains[0];
    setDrafts([
      {
        key: newDraftKey(),
        data_domain_id: defaultDomain?.id,
        training_level: defaultLevelForDomain(defaultDomain),
        grade: '',
        files: [],
        removedFileIds: [],
        uploadedDraftIds: [],
      },
    ]);
    setModalOpen(true);
  };

  const cleanupUploadedDrafts = useCallback(
    async (profileID: number | undefined, currentDrafts: DraftEnrollment[]) => {
      if (!profileID) return;
      const pending = currentDrafts.flatMap((draft) =>
        draft.uploadedDraftIds.map((fileId) => ({ enrollmentId: draft.id, fileId })),
      );
      await Promise.all(
        pending.map(async (item) => {
          if (!item.enrollmentId) return;
          try {
            await alumniApi.deleteDraftFile(profileID, item.enrollmentId, item.fileId);
          } catch {
            // 草稿清理失败不应阻止关闭或重新加载，正式附件不受影响。
          }
        }),
      );
    },
    [],
  );

  const openEditModal = useCallback(
    async (record: AlumniListItem) => {
      await cleanupUploadedDrafts(editing?.id, drafts);
      resetEditor();
      setModalOpen(true);
      setModalLoading(true);
      try {
        const profile = await alumniApi.detail(record.id);
        let fileGroups: AlumniEnrollmentFileGroup[] = [];
        if (filesManageable) {
          const files = await alumniApi.listFiles(record.id, true);
          fileGroups = files.enrollments ?? [];
        }
        setEditing(profile);
        personalForm.setFieldsValue({
          name: profile.name,
          gender: profile.gender,
          mobile: profile.mobile,
          email: profile.email,
          industry: profile.industry,
          work_unit: profile.work_unit,
          position: profile.position,
          mailing_address: profile.mailing_address,
        });
        setDrafts(buildDraftsFromProfile(profile, fileGroups));
      } catch (error) {
        const err = error as Error;
        message.error(err.message || '校友详情加载失败');
        setModalOpen(false);
      } finally {
        setModalLoading(false);
      }
    },
    [
      buildDraftsFromProfile,
      cleanupUploadedDrafts,
      drafts,
      editing?.id,
      filesManageable,
      personalForm,
      resetEditor,
    ],
  );

  const closeModal = useCallback(async () => {
    await cleanupUploadedDrafts(editing?.id, drafts);
    setModalOpen(false);
    resetEditor();
  }, [cleanupUploadedDrafts, drafts, editing?.id, resetEditor]);

  const updateDraft = (key: string, patch: Partial<DraftEnrollment>) => {
    setDrafts((prev) => prev.map((draft) => (draft.key === key ? { ...draft, ...patch } : draft)));
    setFieldErrors((prev) => {
      const next = { ...prev };
      Object.keys(next).forEach((fieldKey) => {
        if (fieldKey.startsWith(`${key}.`)) delete next[fieldKey];
      });
      return next;
    });
  };

  const addEnrollmentDraft = () => {
    setFieldErrors((prev) => {
      if (!prev['enrollments.required']) return prev;
      const next = { ...prev };
      delete next['enrollments.required'];
      return next;
    });
    const defaultDomain = domains[0];
    setDrafts((prev) => [
      ...prev,
      {
        key: newDraftKey(),
        data_domain_id: defaultDomain?.id,
        training_level: defaultLevelForDomain(defaultDomain),
        grade: '',
        files: [],
        removedFileIds: [],
        uploadedDraftIds: [],
      },
    ]);
  };

  const removeEnrollmentDraft = (key: string) => {
    setDrafts((prev) => prev.filter((draft) => draft.key !== key));
  };

  const handleUpload = async (draft: DraftEnrollment, fileType: AlumniFileType, file: File) => {
    if (!editing || !draft.id) {
      message.info('请先保存该段学籍，再上传档案');
      return;
    }
    setUploadingKey(`${draft.key}-${fileType}`);
    try {
      const uploaded = await alumniApi.uploadFile(editing.id, draft.id, fileType, file);
      setDrafts((prev) =>
        prev.map((item) =>
          item.key === draft.key
            ? {
                ...item,
                files: [...item.files, uploaded],
                uploadedDraftIds: [...item.uploadedDraftIds, uploaded.id],
              }
            : item,
        ),
      );
      message.success(`${FILE_TYPE_LABELS[fileType]}已加入草稿，保存全部修改后生效`);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '文件上传失败');
    } finally {
      setUploadingKey(null);
    }
  };

  const removeFile = async (draft: DraftEnrollment, file: AlumniFileItem) => {
    if (!editing || !draft.id) return;
    if (file.status === 'draft') {
      try {
        await alumniApi.deleteDraftFile(editing.id, draft.id, file.id);
        setDrafts((prev) =>
          prev.map((item) =>
            item.key === draft.key
              ? {
                  ...item,
                  files: item.files.filter((candidate) => candidate.id !== file.id),
                  uploadedDraftIds: item.uploadedDraftIds.filter((id) => id !== file.id),
                }
              : item,
          ),
        );
        message.success('草稿文件已删除');
      } catch (error) {
        const err = error as Error;
        message.error(err.message || '草稿文件删除失败');
      }
      return;
    }
    updateDraft(draft.key, { removedFileIds: [...draft.removedFileIds, file.id] });
  };

  const undoRemoveFile = (draft: DraftEnrollment, fileId: number) => {
    updateDraft(draft.key, { removedFileIds: draft.removedFileIds.filter((id) => id !== fileId) });
  };

  const downloadFile = async (draft: DraftEnrollment, file: AlumniFileItem) => {
    if (!editing || !draft.id) return;
    try {
      await alumniApi.downloadFile(editing.id, draft.id, file.id);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '下载失败');
    }
  };

  const validateAll = (values: Record<string, unknown>): FieldErrors => {
    const errors: FieldErrors = {};
    if (!values.name || !String(values.name).trim()) {
      errors.name = '请输入姓名';
    }
    if (drafts.length === 0) {
      errors['enrollments.required'] = '至少需要一段学籍';
    }
    drafts.forEach((draft) => {
      if (!draft.grade.trim()) {
        errors[`${draft.key}.grade`] = '请输入年级';
      }
      if (!draft.data_domain_id) {
        errors[`${draft.key}.data_domain_id`] = '请选择培养类别';
      }
      if (!draft.training_level) {
        errors[`${draft.key}.training_level`] = '请选择培养层次';
      } else if (!draft.id && !EXPLICIT_TRAINING_LEVELS.includes(draft.training_level)) {
        errors[`${draft.key}.training_level`] = '新增学籍需明确选择本科、硕士或博士';
      }
    });
    return errors;
  };

  const handleSave = async () => {
    let values: Record<string, unknown>;
    try {
      values = await personalForm.validateFields();
    } catch {
      setActiveTab('personal');
      return;
    }
    const errors = validateAll(values);
    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors);
      setActiveTab('records');
      message.error('请检查标记的字段后再保存');
      return;
    }
    setFieldErrors({});

    const enrollments: AlumniEnrollmentInput[] = drafts.map((draft) => ({
      ...(draft.id ? { id: draft.id } : {}),
      ...(draft.version ? { version: draft.version } : {}),
      data_domain_id: draft.data_domain_id as number,
      training_level: draft.training_level as TrainingLevel,
      grade: draft.grade.trim(),
      class_name: draft.class_name,
      cohort: draft.cohort,
      counselor: draft.counselor,
      mentor: draft.mentor,
      major: draft.major,
      training_mode: draft.training_mode,
      remark: draft.remark,
      activate_file_ids: draft.files
        .filter((file) => file.status === 'draft')
        .map((file) => file.id),
      remove_file_ids: draft.removedFileIds,
    }));
    const payload: AlumniProfilePayload = {
      ...(editing ? { version: editing.version } : {}),
      name: String(values.name).trim(),
      gender: values.gender as string | undefined,
      mobile: values.mobile as string | undefined,
      email: values.email as string | undefined,
      industry: values.industry as string | undefined,
      work_unit: values.work_unit as string | undefined,
      position: values.position as string | undefined,
      mailing_address: values.mailing_address as string | undefined,
      enrollments,
    };

    setSaving(true);
    try {
      if (editing) {
        await alumniApi.update(editing.id, payload);
        message.success('校友档案已保存');
      } else {
        await alumniApi.create(payload);
        message.success('校友档案已新增');
      }
      setModalOpen(false);
      resetEditor();
      await loadData(query);
    } catch (error) {
      const err = error as Error;
      const conflict = /409|已被其他管理员|重新加载/.test(`${err.message}`);
      if (conflict) {
        Modal.confirm({
          title: '保存冲突',
          content: '该校友的数据已被其他管理员修改。请重新加载最新数据，核对后再保存。',
          okText: '重新加载',
          cancelText: '取消',
          onOk: async () => {
            if (editing) {
              await openEditModal({
                id: editing.id,
                name: editing.name,
                version: editing.version,
                enrollments: [],
              });
            }
          },
        });
      } else {
        message.error(err.message || '保存失败');
      }
    } finally {
      setSaving(false);
    }
  };

  const handleRemove = useCallback(
    async (record: AlumniListItem) => {
      try {
        await alumniApi.remove(record.id);
        message.success('校友档案已删除');
        await loadData(query);
      } catch (error) {
        const err = error as Error;
        message.error(err.message || '删除失败');
      }
    },
    [loadData, query],
  );

  const columns = useMemo<ColumnsType<AlumniListItem>>(
    () => [
      { title: '姓名', dataIndex: 'name', fixed: 'left', width: 120 },
      {
        title: '学籍',
        dataIndex: 'enrollments',
        width: 340,
        render: (_value, record) => (
          <Space direction="vertical" size={2}>
            {(record.enrollments || []).map((enrollment) => (
              <Tag
                key={enrollment.id}
                color={enrollment.training_level === 'unverified' ? 'orange' : 'blue'}
              >
                {enrollmentSummary(enrollment)}
              </Tag>
            ))}
            {(!record.enrollments || record.enrollments.length === 0) && <span>-</span>}
          </Space>
        ),
      },
      { title: '行业', dataIndex: 'industry', width: 130 },
      ...(sensitiveReadable
        ? ([
            { title: '工作单位', dataIndex: 'work_unit', width: 220, ellipsis: true },
            { title: '手机号', dataIndex: 'mobile', width: 140 },
          ] as ColumnsType<AlumniListItem>)
        : []),
      {
        title: '操作',
        key: 'action',
        fixed: 'right',
        width: 160,
        render: (_, record) => (
          <Space size={4}>
            <Button
              type="link"
              icon={<EyeOutlined />}
              onClick={() => navigate(`/alumni/${record.id}`)}
            >
              查看
            </Button>
            <Button type="link" icon={<EditOutlined />} onClick={() => void openEditModal(record)}>
              编辑
            </Button>
            <Popconfirm
              title="删除校友档案"
              description={
                (record.enrollments?.length || 0) > 1
                  ? '该校友有多段学籍，当前不支持整人删除'
                  : '确认删除该校友档案？'
              }
              disabled={(record.enrollments?.length || 0) > 1}
              onConfirm={() => handleRemove(record)}
            >
              <Button
                type="link"
                danger
                icon={<DeleteOutlined />}
                disabled={(record.enrollments?.length || 0) > 1}
              >
                删除
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [handleRemove, navigate, openEditModal, sensitiveReadable],
  );

  const handleSearch = (values: AlumniQuery) => {
    const keyword = values.keyword?.trim();
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      if (keyword) next.set('keyword', keyword);
      else next.delete('keyword');
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

  const handleImportClick = () => fileInputRef.current?.click();

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
      const result = await alumniApi.importData(file, importDomainID);
      setImportResult(result);
      setImportModalOpen(true);
      await loadData(query);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '导入失败');
    } finally {
      setImporting(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const exportMenuItems: MenuProps['items'] = [
    { key: 'xlsx', label: '为 Excel (.xlsx)' },
    { key: 'csv', label: '为 CSV (.csv)' },
  ];

  const renderFileSection = (draft: DraftEnrollment, fileType: AlumniFileType) => {
    const files = draft.files.filter((file) => file.file_type === fileType);
    const uploading = uploadingKey === `${draft.key}-${fileType}`;
    return (
      <Card
        size="small"
        title={
          <Space>
            <Tag color={fileType === 'degree_archive' ? 'blue' : 'green'}>
              {FILE_TYPE_LABELS[fileType]}
            </Tag>
            <span className="text-secondary">
              {files.filter((file) => !draft.removedFileIds.includes(file.id)).length} 个文件
            </span>
          </Space>
        }
        extra={
          <span data-testid={`alumni-file-upload-${fileType}`}>
            <Upload
              accept=".pdf,.doc,.docx,.jpg,.jpeg,.png"
              showUploadList={false}
              beforeUpload={(file) => {
                void handleUpload(draft, fileType, file as File);
                return false;
              }}
              disabled={!draft.id || uploading}
            >
              <Button
                type="link"
                icon={uploading ? <Spin size="small" /> : <FileAddOutlined />}
                disabled={!draft.id || uploading}
              >
                {draft.id ? '上传文件' : '保存后可上传'}
              </Button>
            </Upload>
          </span>
        }
      >
        {files.length === 0 ? (
          <div className="empty-hint">暂无文件</div>
        ) : (
          <Space direction="vertical" style={{ width: '100%' }} size={4}>
            {files.map((file) => {
              const removed = draft.removedFileIds.includes(file.id);
              return (
                <div key={file.id} className="file-row">
                  <span
                    className={
                      removed ? 'file-row__name file-row__name--removed' : 'file-row__name'
                    }
                  >
                    {file.original_name}
                    {file.status === 'draft' ? <Tag color="gold">草稿</Tag> : null}
                    {removed ? <Tag color="red">待移除</Tag> : null}
                  </span>
                  <Space size={4}>
                    <Button
                      type="link"
                      icon={<DownloadOutlined />}
                      disabled={file.status === 'draft' || removed}
                      onClick={() => void downloadFile(draft, file)}
                    >
                      下载
                    </Button>
                    {removed ? (
                      <Button type="link" onClick={() => undoRemoveFile(draft, file.id)}>
                        撤销移除
                      </Button>
                    ) : (
                      <Button
                        type="link"
                        danger
                        icon={<DeleteOutlined />}
                        onClick={() => void removeFile(draft, file)}
                      >
                        移除
                      </Button>
                    )}
                  </Space>
                </div>
              );
            })}
          </Space>
        )}
      </Card>
    );
  };

  const renderEnrollmentEditor = (draft: DraftEnrollment, index: number) => {
    const canUpload = Boolean(draft.id);
    return (
      <Card
        key={draft.key}
        size="small"
        className="enrollment-editor"
        title={
          <Space>
            <span>
              {levelLabel(draft.training_level || undefined)}学籍{' '}
              {draft.id ? '' : <Tag color="green">新增</Tag>}
            </span>
            <span className="text-secondary">
              {draft.grade ? `${draft.grade}级` : '年级未填写'}
            </span>
          </Space>
        }
        extra={
          draft.id ? null : (
            <Button type="link" danger onClick={() => removeEnrollmentDraft(draft.key)}>
              移除本次新增
            </Button>
          )
        }
      >
        <div className="modal-grid">
          <Form.Item
            label="培养类别"
            required
            validateStatus={fieldErrors[`${draft.key}.data_domain_id`] ? 'error' : undefined}
            help={fieldErrors[`${draft.key}.data_domain_id`]}
          >
            <Select
              value={draft.data_domain_id}
              disabled={Boolean(draft.id)}
              onChange={(value) => updateDraft(draft.key, { data_domain_id: value })}
              options={domains.map((domain) => ({ label: domain.name, value: domain.id }))}
            />
          </Form.Item>
          <Form.Item
            label="培养层次"
            required
            validateStatus={fieldErrors[`${draft.key}.training_level`] ? 'error' : undefined}
            help={fieldErrors[`${draft.key}.training_level`]}
          >
            <Select
              value={draft.training_level || undefined}
              onChange={(value: TrainingLevel) => updateDraft(draft.key, { training_level: value })}
              options={
                draft.id
                  ? Object.entries(TRAINING_LEVEL_LABELS).map(([value, label]) => ({
                      value,
                      label,
                    }))
                  : EXPLICIT_TRAINING_LEVELS.map((value) => ({
                      value,
                      label: TRAINING_LEVEL_LABELS[value],
                    }))
              }
              data-testid={`enrollment-level-${index}`}
            />
          </Form.Item>
          <Form.Item
            label="年级"
            required
            validateStatus={fieldErrors[`${draft.key}.grade`] ? 'error' : undefined}
            help={fieldErrors[`${draft.key}.grade`]}
          >
            <Input
              value={draft.grade}
              maxLength={50}
              onChange={(event) => updateDraft(draft.key, { grade: event.target.value })}
              data-testid={`enrollment-grade-${index}`}
            />
          </Form.Item>
          <Form.Item label="班级">
            <Input
              value={draft.class_name || ''}
              maxLength={100}
              onChange={(event) => updateDraft(draft.key, { class_name: event.target.value })}
            />
          </Form.Item>
          <Form.Item label="届数">
            <Input
              value={draft.cohort || ''}
              maxLength={50}
              onChange={(event) => updateDraft(draft.key, { cohort: event.target.value })}
            />
          </Form.Item>
          <Form.Item label="专业">
            <Input
              value={draft.major || ''}
              maxLength={100}
              onChange={(event) => updateDraft(draft.key, { major: event.target.value })}
            />
          </Form.Item>
          <Form.Item label="培养方式">
            <Select
              allowClear
              value={draft.training_mode}
              onChange={(value) => updateDraft(draft.key, { training_mode: value })}
              options={trainingModeOptions.map((value) => ({ label: value, value }))}
            />
          </Form.Item>
          <Form.Item label="辅导员">
            <Input
              value={draft.counselor || ''}
              maxLength={100}
              onChange={(event) => updateDraft(draft.key, { counselor: event.target.value })}
            />
          </Form.Item>
          <Form.Item label="导师">
            <Input
              value={draft.mentor || ''}
              maxLength={100}
              onChange={(event) => updateDraft(draft.key, { mentor: event.target.value })}
            />
          </Form.Item>
        </div>
        <Form.Item label="管理员备注">
          <Input.TextArea
            rows={2}
            value={draft.remark || ''}
            onChange={(event) => updateDraft(draft.key, { remark: event.target.value })}
          />
        </Form.Item>
        {filesManageable ? (
          <>
            <h4>学位档案与学籍档案</h4>
            {!canUpload && (
              <div className="empty-hint">保存该段学籍后可上传档案文件，草稿在保存前不会生效。</div>
            )}
            <Space direction="vertical" style={{ width: '100%' }} size="small">
              {renderFileSection(draft, 'degree_archive')}
              {renderFileSection(draft, 'academic_record')}
            </Space>
          </>
        ) : null}
      </Card>
    );
  };

  return (
    <>
      <PageHeader
        title="校友管理"
        description="管理员维护可管理范围内的个人主档案与多段学籍"
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
          <Form.Item name="major">
            <Input allowClear placeholder="专业" />
          </Form.Item>
          <Form.Item name="training_level">
            <Select
              allowClear
              placeholder="培养层次"
              style={{ width: 120 }}
              options={Object.entries(TRAINING_LEVEL_LABELS).map(([value, label]) => ({
                value,
                label,
              }))}
            />
          </Form.Item>
          <Form.Item name="industry">
            <Select
              allowClear
              placeholder="行业"
              options={industryOptions.map((value) => ({ label: value, value }))}
            />
          </Form.Item>
          <Form.Item name="data_domain_id">
            <Select
              allowClear
              placeholder="培养类别"
              options={domains.map((domain) => ({ label: domain.name, value: domain.id }))}
            />
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" icon={<SearchOutlined />}>
              查询
            </Button>
            <Button icon={<UndoOutlined />} onClick={handleReset}>
              重置
            </Button>
            <Select
              value={importDomainID}
              onChange={setImportDomainID}
              allowClear
              placeholder="导入目标培养类别"
              style={{ width: 170 }}
              options={domains.map((domain) => ({ label: domain.name, value: domain.id }))}
            />
            <Button icon={<UploadOutlined />} loading={importing} onClick={handleImportClick}>
              导入 Excel
            </Button>
            <Button icon={<FileTextOutlined />} onClick={handleDownloadTemplate}>
              导出模板
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              accept=".xlsx,.csv"
              style={{ display: 'none' }}
              onChange={handleFileChange}
            />
            <Dropdown menu={{ items: exportMenuItems, onClick: ({ key }) => handleExport(key) }}>
              <Button icon={<DownloadOutlined />}>导出</Button>
            </Dropdown>
          </Space>
        </Form>
      </Card>
      <Card className="tool-card">
        <Table<AlumniListItem>
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={items}
          scroll={{ x: 1200 }}
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
        onCancel={() => void closeModal()}
        onOk={() => void handleSave()}
        okText="保存全部修改"
        cancelText="取消"
        confirmLoading={saving}
        width={980}
        destroyOnClose
        maskClosable={false}
      >
        <Spin spinning={modalLoading}>
          {Object.keys(fieldErrors).length > 0 && (
            <Alert
              type="error"
              showIcon
              message="请检查以下内容"
              description="必填字段未填写或培养层次无效。"
              style={{ marginBottom: 12 }}
            />
          )}
          <Tabs
            activeKey={activeTab}
            onChange={setActiveTab}
            items={[
              {
                key: 'personal',
                label: '个人信息',
                children: (
                  <Form form={personalForm} layout="vertical" className="modal-grid">
                    <Form.Item
                      label="姓名"
                      name="name"
                      rules={[{ required: true, message: '请输入姓名' }]}
                      validateStatus={fieldErrors.name ? 'error' : undefined}
                      help={fieldErrors.name}
                    >
                      <Input data-testid="alumni-name-input" maxLength={100} />
                    </Form.Item>
                    <Form.Item label="性别" name="gender">
                      <Select
                        allowClear
                        options={genderOptions.map((value) => ({ label: value, value }))}
                      />
                    </Form.Item>
                    {sensitiveReadable ? (
                      <Form.Item label="手机号" name="mobile">
                        <Input maxLength={30} />
                      </Form.Item>
                    ) : null}
                    {sensitiveReadable ? (
                      <Form.Item label="邮箱" name="email">
                        <Input maxLength={255} />
                      </Form.Item>
                    ) : null}
                    <Form.Item label="行业" name="industry">
                      <Select
                        allowClear
                        options={industryOptions.map((value) => ({ label: value, value }))}
                      />
                    </Form.Item>
                    {sensitiveReadable ? (
                      <Form.Item label="工作单位" name="work_unit">
                        <Input maxLength={255} />
                      </Form.Item>
                    ) : null}
                    {sensitiveReadable ? (
                      <Form.Item label="职务" name="position">
                        <Input maxLength={100} />
                      </Form.Item>
                    ) : null}
                    {sensitiveReadable ? (
                      <Form.Item
                        label="通讯地址"
                        name="mailing_address"
                        className="modal-grid-wide"
                      >
                        <Input.TextArea rows={2} maxLength={255} showCount />
                      </Form.Item>
                    ) : null}
                    <div className="modal-grid-wide empty-hint">
                      个人信息由本人共用一份；修改一次即可同步到全部学籍。联系方式变化不会自动改变登录账号。
                    </div>
                  </Form>
                ),
              },
              {
                key: 'records',
                label: '学籍与档案',
                children: (
                  <div className="enrollment-editor-list">
                    {drafts.map((draft, index) => renderEnrollmentEditor(draft, index))}
                    <Button
                      type="dashed"
                      icon={<PlusOutlined />}
                      onClick={addEnrollmentDraft}
                      block
                    >
                      添加一段学籍
                    </Button>
                    <div className="empty-hint">
                      同一校友可添加多段本科、硕士或博士学籍；未提交的学籍不会被删除。
                    </div>
                  </div>
                ),
              },
            ]}
          />
        </Spin>
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
            {importResult.errors.length > 0 ? (
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
            ) : (
              <p style={{ color: '#52c41a', marginTop: 8 }}>全部导入成功，无错误记录</p>
            )}
          </div>
        )}
      </Modal>
    </>
  );
}
