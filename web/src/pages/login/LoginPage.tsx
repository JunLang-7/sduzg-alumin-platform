import { LockOutlined, LoginOutlined, UserOutlined } from '@ant-design/icons';
import { Button, Form, Input, Space, Typography, message } from 'antd';
import { useLocation, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../../store/authStore';
import { getDefaultPath } from '../../utils/permissions';
import type { LoginRequest } from '../../types/auth';

const { Title, Text, Paragraph } = Typography;

interface LocationState {
  from?: {
    pathname?: string;
  };
}

export function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const login = useAuthStore((state) => state.login);
  const loading = useAuthStore((state) => state.loading);

  const onFinish = async (values: LoginRequest) => {
    try {
      const user = await login(values);
      const state = location.state as LocationState | null;
      navigate(state?.from?.pathname || getDefaultPath(user.role), { replace: true });
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '登录失败');
    }
  };

  return (
    <main className="login-page">
      <section className="login-brand">
        <span className="login-badge">MPA 试点</span>
        <Title>山大政管学院校友平台</Title>
        <Paragraph>面向 MPA 校友档案维护、校友查询与学院管理的数据工作台。</Paragraph>
        <Space className="login-kpis" size={12} wrap>
          <span>
            <strong>统一入口</strong>
            <Text>校友 / 管理员</Text>
          </span>
          <span>
            <strong>资料维护</strong>
            <Text>信息可追踪</Text>
          </span>
          <span>
            <strong>权限分层</strong>
            <Text>角色边界清晰</Text>
          </span>
        </Space>
      </section>
      <section className="login-panel" aria-label="登录表单">
        <div className="login-card">
          <Title level={3}>账号登录</Title>
          <Text className="login-card-note" type="secondary">
            请使用学院分配的账号进入平台。
          </Text>
          <Form<LoginRequest> layout="vertical" size="large" onFinish={onFinish}>
            <Form.Item
              label="账号"
              name="account"
              rules={[{ required: true, message: '请输入账号' }]}
            >
              <Input prefix={<UserOutlined />} autoComplete="username" />
            </Form.Item>
            <Form.Item
              label="密码"
              name="password"
              rules={[{ required: true, message: '请输入密码' }]}
            >
              <Input.Password prefix={<LockOutlined />} autoComplete="current-password" />
            </Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              icon={<LoginOutlined />}
              loading={loading}
              block
            >
              登录
            </Button>
          </Form>
        </div>
      </section>
    </main>
  );
}
