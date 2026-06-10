import { LockOutlined, LoginOutlined, MailOutlined, MobileOutlined, UserOutlined } from '@ant-design/icons';
import { Button, Form, Input, Modal, Space, Tabs, Typography, message } from 'antd';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { authApi } from '../../api/auth';
import { cacheAccessToken } from '../../api/http';
import { useAuthStore } from '../../store/authStore';
import { getDefaultPath } from '../../utils/permissions';
import type { CurrentUser, LoginRequest, SetupPasswordRequest } from '../../types/auth';

const { Title, Text, Paragraph } = Typography;

interface LocationState {
  from?: {
    pathname?: string;
  };
}

function normalizeLoginInput(raw: string): Partial<LoginRequest> {
  const trimmed = raw.trim();
  if (/^1[3-9]\d{9}$/.test(trimmed)) return { mobile: trimmed };
  if (/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmed)) return { email: trimmed };
  return { account: trimmed };
}

interface CountdownButtonProps {
  onClick: () => Promise<void>;
  children: React.ReactNode;
}

function CountdownButton({ onClick, children }: CountdownButtonProps) {
  const [loading, setLoading] = useState(false);
  const [countdown, setCountdown] = useState(0);
  const timerRef = useRef<ReturnType<typeof setInterval>>();

  useEffect(() => {
    if (countdown > 0) {
      timerRef.current = setInterval(() => {
        setCountdown((prev) => prev - 1);
      }, 1000);
    }
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [countdown]);

  const handleClick = async () => {
    if (loading || countdown > 0) return;
    setLoading(true);
    try {
      await onClick();
      setCountdown(60);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '验证码发送失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Button onClick={handleClick} disabled={countdown > 0} loading={loading} size="large">
      {countdown > 0 ? `${countdown}s` : children}
    </Button>
  );
}

export function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const login = useAuthStore((state) => state.login);
  const loading = useAuthStore((state) => state.loading);
  const [activeTab, setActiveTab] = useState('password');
  const [passwordForm] = Form.useForm();
  const [smsForm] = Form.useForm();
  const [emailForm] = Form.useForm();
  const [setupModalOpen, setSetupModalOpen] = useState(false);
  const [setupForm] = Form.useForm<SetupPasswordRequest>();
  const [setupLoading, setSetupLoading] = useState(false);
  const [registrationToken, setRegistrationToken] = useState<string | null>(null);

  const navigateAfterLogin = useCallback((user: CurrentUser) => {
    const state = location.state as LocationState | null;
    navigate(state?.from?.pathname || getDefaultPath(user.role), { replace: true });
  }, [location.state, navigate]);

  const handleCodeLogin = useCallback(async (loginPayload: LoginRequest) => {
    const { user, registrationToken: regToken } = await login(loginPayload);
    if (regToken) {
      setRegistrationToken(regToken);
      setupForm.resetFields();
      setSetupModalOpen(true);
    } else if (user) {
      navigateAfterLogin(user);
    }
  }, [login, setupForm, navigateAfterLogin]);

  const onPasswordFinish = useCallback(async (values: LoginRequest) => {
    try {
      const { user } = await login({ ...values });
      if (user) {
        navigateAfterLogin(user);
      }
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '登录失败');
    }
  }, [login, navigateAfterLogin]);

  const onSmsFinish = useCallback(async (values: Record<string, string>) => {
    try {
      await handleCodeLogin({
        mobile: values.phone,
        code: values.code,
        grant_type: 'sms_code',
      });
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '登录失败');
    }
  }, [handleCodeLogin]);

  const onEmailFinish = useCallback(async (values: Record<string, string>) => {
    try {
      await handleCodeLogin({
        email: values.email,
        code: values.code,
        grant_type: 'email_code',
      });
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '登录失败');
    }
  }, [handleCodeLogin]);

  const onSetupPassword = useCallback(async () => {
    try {
      const values = await setupForm.validateFields();
      if (!registrationToken) return;
      setSetupLoading(true);
      const result = await authApi.setupPassword({
        ...values,
        registration_token: registrationToken,
      });
      cacheAccessToken(result.access_token);
      useAuthStore.setState({ user: result.user, sessionChecked: true });
      setSetupModalOpen(false);
      message.success('密码设置成功');
      navigateAfterLogin(result.user);
    } catch (error) {
      const err = error as Error;
      if (err.message) {
        message.error(err.message || '密码设置失败');
      }
    } finally {
      setSetupLoading(false);
    }
  }, [setupForm, registrationToken, navigateAfterLogin]);

  const sendSmsCode = useCallback(async () => {
    const phone = smsForm.getFieldValue('phone');
    if (!phone || !/^1[3-9]\d{9}$/.test(phone)) {
      throw new Error('请输入正确的手机号');
    }
    await authApi.sendVerifyCode({ target: phone, purpose: 'login' });
    message.success('验证码已发送');
  }, [smsForm]);

  const sendEmailCode = useCallback(async () => {
    const email = emailForm.getFieldValue('email');
    if (!email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      throw new Error('请输入正确的邮箱');
    }
    await authApi.sendVerifyCode({ target: email, purpose: 'login' });
    message.success('验证码已发送');
  }, [emailForm]);

  const tabItems = [
    {
      key: 'password',
      label: '密码登录',
      children: (
        <Form<LoginRequest> form={passwordForm} layout="vertical" size="large" onFinish={onPasswordFinish}>
          <Form.Item
            label="用户名"
            name="account"
            rules={[{ required: true, message: '请输入用户名' }]}
          >
            <Input prefix={<UserOutlined />} autoComplete="username" placeholder="手机号/邮箱/账号" />
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
      ),
    },
    {
      key: 'sms_code',
      label: '短信验证码登录',
      children: (
        <Form form={smsForm} layout="vertical" size="large" onFinish={onSmsFinish}>
          <Form.Item
            label="手机号"
            name="phone"
            rules={[
              { required: true, message: '请输入手机号' },
              { pattern: /^1[3-9]\d{9}$/, message: '手机号格式不正确' },
            ]}
          >
            <Input prefix={<MobileOutlined />} placeholder="请输入手机号" autoComplete="tel" />
          </Form.Item>
          <Form.Item
            label="验证码"
            name="code"
            rules={[
              { required: true, message: '请输入验证码' },
              { len: 6, message: '验证码为6位数字' },
            ]}
          >
            <Input
              prefix={<LockOutlined />}
              placeholder="6位数字验证码"
              maxLength={6}
              suffix={
                <CountdownButton onClick={sendSmsCode}>获取验证码</CountdownButton>
              }
            />
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
      ),
    },
    {
      key: 'email_code',
      label: '邮件验证码登录',
      children: (
        <Form form={emailForm} layout="vertical" size="large" onFinish={onEmailFinish}>
          <Form.Item
            label="邮箱"
            name="email"
            rules={[
              { required: true, message: '请输入邮箱' },
              { type: 'email', message: '邮箱格式不正确' },
            ]}
          >
            <Input prefix={<MailOutlined />} placeholder="请输入邮箱" autoComplete="email" />
          </Form.Item>
          <Form.Item
            label="验证码"
            name="code"
            rules={[
              { required: true, message: '请输入验证码' },
              { len: 6, message: '验证码为6位数字' },
            ]}
          >
            <Input
              prefix={<LockOutlined />}
              placeholder="6位数字验证码"
              maxLength={6}
              suffix={
                <CountdownButton onClick={sendEmailCode}>获取验证码</CountdownButton>
              }
            />
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
      ),
    },
  ];

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
          <Title level={3}>校友登录</Title>
          <Tabs activeKey={activeTab} onChange={setActiveTab} centered items={tabItems} />
        </div>
      </section>

      <Modal
        title="请设置您的登录密码"
        open={setupModalOpen}
        onOk={onSetupPassword}
        confirmLoading={setupLoading}
        okText="设置密码"
        cancelButtonProps={{ style: { display: 'none' } }}
        closable={false}
        maskClosable={false}
        keyboard={false}
      >
        <Form form={setupForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            label="新密码"
            name="new_password"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 8, message: '密码至少8位' },
              {
                pattern: /^(?=.*[a-zA-Z])(?=.*\d)/,
                message: '密码必须包含字母和数字',
              },
            ]}
          >
            <Input.Password placeholder="至少8位" />
          </Form.Item>
          <Form.Item
            label="确认密码"
            name="confirm_password"
            dependencies={['new_password']}
            rules={[
              { required: true, message: '请确认密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('new_password') === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error('两次输入的密码不一致'));
                },
              }),
            ]}
          >
            <Input.Password placeholder="再次输入密码" />
          </Form.Item>
        </Form>
      </Modal>
    </main>
  );
}
