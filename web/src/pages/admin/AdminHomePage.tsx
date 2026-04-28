import { useNavigate } from 'react-router-dom';
import { BarChartOutlined, TeamOutlined, UserOutlined } from '@ant-design/icons';
import { Button, Card, Col, Row, Typography } from 'antd';
import { PageHeader } from '../../components/PageHeader';
import { useAuthStore } from '../../store/authStore';
import { hasRole } from '../../utils/permissions';

const { Text } = Typography;

export function AdminHomePage() {
  const navigate = useNavigate();
  const user = useAuthStore((state) => state.user);

  return (
    <>
      <PageHeader title="管理后台首页" description="校友档案维护、统计分析和管理员账号管理" />
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={8}>
          <Card className="action-card">
            <TeamOutlined />
            <h3>校友管理</h3>
            <Text type="secondary">新增、编辑、删除校友档案，维护账号绑定关系。</Text>
            <Button type="primary" onClick={() => navigate('/admin/alumni')}>
              进入管理
            </Button>
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card className="action-card">
            <BarChartOutlined />
            <h3>数据大屏</h3>
            <Text type="secondary">查看校友规模、分布和资料完整率。</Text>
            <Button type="primary" onClick={() => navigate('/admin/dashboard')}>
              查看大屏
            </Button>
          </Card>
        </Col>
        {hasRole(user, 'super_admin') ? (
          <Col xs={24} lg={8}>
            <Card className="action-card">
              <UserOutlined />
              <h3>管理员管理</h3>
              <Text type="secondary">创建、删除运营管理员账号。</Text>
              <Button type="primary" onClick={() => navigate('/admin/users')}>
                管理账号
              </Button>
            </Card>
          </Col>
        ) : null}
      </Row>
    </>
  );
}
