import { LockOutlined, SaveOutlined } from '@ant-design/icons';
import { Button, Card, Form, Input, message } from 'antd';
import { useNavigate } from 'react-router-dom';
import { PageHeader } from '../../components/PageHeader';
import { useAuthStore } from '../../store/authStore';
import type { ChangePasswordRequest } from '../../types/auth';

export function ChangePasswordPage() {
  const [form] = Form.useForm<ChangePasswordRequest>();
  const navigate = useNavigate();
  const changePassword = useAuthStore((state) => state.changePassword);
  const loading = useAuthStore((state) => state.loading);

  const onFinish = async (values: ChangePasswordRequest) => {
    try {
      await changePassword(values);
      message.success('密码已修改');
      navigate('/');
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '密码修改失败');
    }
  };

  return (
    <>
      <PageHeader title="修改密码" />
      <Card className="tool-card form-card">
        <Form form={form} layout="vertical" onFinish={onFinish}>
          <Form.Item
            label="原密码"
            name="old_password"
            rules={[{ required: true, message: '请输入原密码' }]}
          >
            <Input.Password prefix={<LockOutlined />} autoComplete="current-password" />
          </Form.Item>
          <Form.Item
            label="新密码"
            name="new_password"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 8, message: '密码至少 8 位' },
            ]}
          >
            <Input.Password prefix={<LockOutlined />} autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            label="确认新密码"
            name="confirm_password"
            dependencies={['new_password']}
            rules={[
              { required: true, message: '请再次输入新密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('new_password') === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error('两次输入的新密码不一致'));
                },
              }),
            ]}
          >
            <Input.Password prefix={<LockOutlined />} autoComplete="new-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={loading}>
            保存密码
          </Button>
        </Form>
      </Card>
    </>
  );
}
