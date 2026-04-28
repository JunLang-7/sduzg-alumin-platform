import type { ReactNode } from 'react';
import { Space, Typography } from 'antd';

const { Title, Text } = Typography;

interface PageHeaderProps {
  title: string;
  description?: string;
  extra?: ReactNode;
}

export function PageHeader({ title, description, extra }: PageHeaderProps) {
  return (
    <div className="page-header">
      <div>
        <Title level={3}>{title}</Title>
        {description ? <Text type="secondary">{description}</Text> : null}
      </div>
      {extra ? <Space>{extra}</Space> : null}
    </div>
  );
}
