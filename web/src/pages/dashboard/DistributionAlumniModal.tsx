import { useEffect, useMemo, useState } from 'react';
import { SearchOutlined } from '@ant-design/icons';
import { Input, Modal, Spin } from 'antd';
import type { AlumniProfile } from '../../types/alumni';

interface DistributionAlumniModalProps {
  open: boolean;
  loading: boolean;
  title: string;
  items: AlumniProfile[];
  onClose: () => void;
  onSelect: (profile: AlumniProfile) => void;
}

function displayValue(value?: string) {
  return value?.trim() || '未填';
}

export function DistributionAlumniModal({
  open,
  loading,
  title,
  items,
  onClose,
  onSelect,
}: DistributionAlumniModalProps) {
  const [keyword, setKeyword] = useState('');

  useEffect(() => {
    if (open) {
      setKeyword('');
    }
  }, [open, title]);

  const filteredItems = useMemo(() => {
    const normalized = keyword.trim().toLowerCase();
    if (!normalized) {
      return items;
    }

    return items.filter((item) =>
      [
        item.name,
        item.grade,
        item.class_name,
        item.cohort,
        item.major,
        item.industry,
        item.work_unit,
        item.position,
        item.mentor,
        item.counselor,
        item.mobile,
      ].some((value) => value?.toLowerCase().includes(normalized)),
    );
  }, [items, keyword]);

  return (
    <Modal
      centered
      footer={null}
      open={open}
      width="min(1120px, 95vw)"
      className="dashboard-distribution-modal"
      title={`${title} · ${items.length} 人`}
      onCancel={onClose}
      destroyOnHidden
    >
      <Spin spinning={loading}>
        <div className="distribution-alumni-search">
          <Input
            allowClear
            prefix={<SearchOutlined />}
            value={keyword}
            placeholder="在当前筛选结果中检索姓名、单位、行业、导师等..."
            onChange={(event) => setKeyword(event.target.value)}
          />
          <span>
            {keyword.trim() ? `当前匹配 ${filteredItems.length} 人` : `当前共 ${items.length} 人`}
          </span>
        </div>
        <div className="distribution-alumni-list">
          {filteredItems.map((item) => (
            <button type="button" key={item.id} onClick={() => onSelect(item)}>
              <strong>{item.name}</strong>
              <span>{displayValue(item.grade)}</span>
              <span>{displayValue(item.industry)}</span>
              <em>{displayValue(item.work_unit)}</em>
              <small>{displayValue(item.mentor)}</small>
            </button>
          ))}
          {!loading && !filteredItems.length ? (
            <div className="distribution-alumni-empty">
              {keyword.trim() ? '当前筛选结果中没有匹配人员' : '该分布项暂无校友信息'}
            </div>
          ) : null}
        </div>
      </Spin>
    </Modal>
  );
}
