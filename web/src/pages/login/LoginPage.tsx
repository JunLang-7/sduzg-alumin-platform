import { LockOutlined, LoginOutlined, UserOutlined } from '@ant-design/icons';
import { Button, Form, Input, Typography, message } from 'antd';
import { useLocation, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../../store/authStore';
import { getDefaultPath } from '../../utils/permissions';
import type { LoginRequest } from '../../types/auth';

const { Title, Text } = Typography;

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
        <Text>校友数据管理与查询</Text>
      </section>
      <section className="login-panel" aria-label="登录表单">
        <div className="login-card">
          <Title level={3}>账号登录</Title>
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
