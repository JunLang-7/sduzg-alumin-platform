import { Tag } from 'antd';

interface StatusTextProps {
  value?: string;
}

export function StatusText({ value }: StatusTextProps) {
  if (value === 'active') {
    return <Tag color="success">正常</Tag>;
  }

  if (value === 'disabled') {
    return <Tag color="warning">停用</Tag>;
  }

  if (value === 'deleted') {
    return <Tag color="default">已删除</Tag>;
  }

  return <Tag>未知</Tag>;
}
