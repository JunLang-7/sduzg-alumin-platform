import { MailOutlined, MobileOutlined, SaveOutlined } from '@ant-design/icons';
import { Button, Card, Form, Input, Modal, Spin, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { authApi } from '../../api/auth';
import { alumniApi } from '../../api/alumni';
import { PageHeader } from '../../components/PageHeader';
import type { AlumniProfile, MyProfilePayload } from '../../types/alumni';

const { Text } = Typography;

function maskPhone(phone?: string): string {
  if (!phone || phone.length < 7) return phone || '未绑定';
  return phone.slice(0, 3) + '****' + phone.slice(-4);
}

function maskEmail(email?: string): string {
  if (!email) return '未绑定';
  const [local, domain] = email.split('@');
  if (!domain) return email;
  const masked = local.charAt(0) + '***';
  return `${masked}@${domain}`;
}

export function ProfilePage() {
  const [form] = Form.useForm<MyProfilePayload>();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [profile, setProfile] = useState<AlumniProfile | null>(null);
  const [contactModalOpen, setContactModalOpen] = useState(false);
  const [contactForm] = Form.useForm();
  const [sendingCode, setSendingCode] = useState(false);
  const [codeCountdown, setCodeCountdown] = useState(0);

  useEffect(() => {
    setLoading(true);
    alumniApi
      .myProfile()
      .then((data) => {
        setProfile(data);
        form.setFieldsValue({
          work_unit: data.work_unit,
          position: data.position,
          mailing_address: data.mailing_address,
          mobile: data.mobile,
        });
      })
      .catch((error: Error) => message.error(error.message || '个人资料加载失败'))
      .finally(() => setLoading(false));
  }, [form]);

  const onFinish = async (values: MyProfilePayload) => {
    setSaving(true);
    try {
      const updated = await alumniApi.updateMyProfile(values);
      setProfile(updated);
      message.success('资料已保存');
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '资料保存失败');
    } finally {
      setSaving(false);
    }
  };

  const phoneBound = !!profile?.mobile;
  const emailBound = !!profile?.email;

  const sendContactCode = useCallback(async () => {
    const newMobile = contactForm.getFieldValue('mobile');
    const newEmail = contactForm.getFieldValue('email');
    const target = newMobile || newEmail;
    if (!target) {
      message.error('请输入要绑定的手机号或邮箱');
      return;
    }
    setSendingCode(true);
    try {
      await authApi.sendVerifyCode({ target, purpose: 'login' });
      message.success('验证码已发送');
      setCodeCountdown(60);
      const timer = setInterval(() => {
        setCodeCountdown((prev) => {
          if (prev <= 1) clearInterval(timer);
          return prev - 1;
        });
      }, 1000);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '验证码发送失败');
    } finally {
      setSendingCode(false);
    }
  }, [contactForm]);

  const updateContact = useCallback(async () => {
    const values = await contactForm.validateFields();
    setSaving(true);
    try {
      await alumniApi.updateContact({
        mobile: values.mobile || undefined,
        email: values.email || undefined,
        code: values.code,
      });
      message.success('联系方式已更新');
      setContactModalOpen(false);
      const updated = await alumniApi.myProfile();
      setProfile(updated);
    } catch (error) {
      const err = error as Error;
      message.error(err.message || '更新失败');
    } finally {
      setSaving(false);
    }
  }, [contactForm]);

  const openContactModal = () => {
    contactForm.resetFields();
    setContactModalOpen(true);
  };

  return (
    <>
      <PageHeader
        title="我的资料"
        description="维护工作单位、职务、通讯地址和联系方式"
      />
      <Card className="tool-card form-card">
        <Spin spinning={loading}>
          <div className="profile-summary">
            <span>{profile?.name || '-'}</span>
            <span>{profile?.grade || '-'}</span>
            <span>{profile?.class_name || '-'}</span>
          </div>

          {/* Contact Info Section */}
          <div style={{ marginBottom: 24, padding: 16, background: '#fafafa', borderRadius: 8 }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
              <Text strong>联系方式</Text>
              {(!phoneBound || !emailBound) && (
                <Button size="small" onClick={openContactModal}>绑定</Button>
              )}
            </div>
            <div style={{ display: 'flex', gap: 24 }}>
              <div>
                <MobileOutlined style={{ marginRight: 4 }} />
                <Text type="secondary">手机号：</Text>
                <Text>{maskPhone(profile?.mobile)}</Text>
              </div>
              <div>
                <MailOutlined style={{ marginRight: 4 }} />
                <Text type="secondary">邮箱：</Text>
                <Text>{maskEmail(profile?.email)}</Text>
              </div>
            </div>
          </div>

          <Form form={form} layout="vertical" onFinish={onFinish}>
            <Form.Item label="工作单位" name="work_unit">
              <Input maxLength={255} />
            </Form.Item>
            <Form.Item label="职务" name="position">
              <Input maxLength={100} />
            </Form.Item>
            <Form.Item label="手机号" name="mobile">
              <Input maxLength={30} />
            </Form.Item>
            <Form.Item label="通讯地址" name="mailing_address">
              <Input.TextArea rows={4} maxLength={255} showCount />
            </Form.Item>
            <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving}>
              保存资料
            </Button>
          </Form>
        </Spin>
      </Card>

      {/* Contact Edit Modal — only shows fields not yet bound */}
      <Modal
        title="绑定联系方式"
        open={contactModalOpen}
        onCancel={() => setContactModalOpen(false)}
        onOk={updateContact}
        confirmLoading={saving}
        okText="绑定"
        cancelText="取消"
      >
        <Form form={contactForm} layout="vertical">
          {phoneBound ? (
            <div style={{ marginBottom: 16, display: 'flex', alignItems: 'center', gap: 8 }}>
              <MobileOutlined />
              <Text type="secondary">手机号：</Text>
              <Text>{maskPhone(profile?.mobile)}</Text>
              <Text type="secondary" style={{ fontSize: 12 }}>（已绑定）</Text>
            </div>
          ) : (
            <Form.Item
              label="手机号"
              name="mobile"
              rules={[
                { required: true, message: '请输入手机号' },
                { pattern: /^1[3-9]\d{9}$/, message: '手机号格式不正确' },
              ]}
            >
              <Input prefix={<MobileOutlined />} placeholder="请输入手机号" maxLength={11} />
            </Form.Item>
          )}

          {emailBound ? (
            <div style={{ marginBottom: 16, display: 'flex', alignItems: 'center', gap: 8 }}>
              <MailOutlined />
              <Text type="secondary">邮箱：</Text>
              <Text>{maskEmail(profile?.email)}</Text>
              <Text type="secondary" style={{ fontSize: 12 }}>（已绑定）</Text>
            </div>
          ) : (
            <Form.Item
              label="邮箱"
              name="email"
              rules={[
                { required: true, message: '请输入邮箱' },
                { type: 'email', message: '邮箱格式不正确' },
              ]}
            >
              <Input prefix={<MailOutlined />} placeholder="请输入邮箱" />
            </Form.Item>
          )}

          {(phoneBound && emailBound) ? (
            <Text type="secondary">手机号和邮箱均已绑定。</Text>
          ) : (
            <Form.Item
              label="验证码"
              name="code"
              rules={[
                { required: true, message: '请输入验证码' },
                { len: 6, message: '验证码为6位数字' },
              ]}
            >
              <Input
                placeholder="6位数字验证码"
                maxLength={6}
                suffix={
                  <Button
                    size="small"
                    onClick={sendContactCode}
                    disabled={codeCountdown > 0}
                    loading={sendingCode}
                  >
                    {codeCountdown > 0 ? `${codeCountdown}s` : '获取验证码'}
                  </Button>
                }
              />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </>
  );
}
