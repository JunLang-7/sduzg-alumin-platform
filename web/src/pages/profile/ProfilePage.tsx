import { useEffect, useState } from 'react';
import { SaveOutlined } from '@ant-design/icons';
import { Button, Card, Form, Input, Spin, message } from 'antd';
import { alumniApi } from '../../api/alumni';
import { PageHeader } from '../../components/PageHeader';
import type { AlumniProfile, MyProfilePayload } from '../../types/alumni';

export function ProfilePage() {
  const [form] = Form.useForm<MyProfilePayload>();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [profile, setProfile] = useState<AlumniProfile | null>(null);

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

  return (
    <>
      <PageHeader
        title="我的资料"
        description="维护工作单位、职务、通讯地址和联系方式"
      />
      <Card className="tool-card form-card">
        <Spin spinning={loading}>
          <Form form={form} layout="vertical" onFinish={onFinish}>
            <div className="profile-summary">
              <span>{profile?.name || '-'}</span>
              <span>{profile?.grade || '-'}</span>
              <span>{profile?.class_name || '-'}</span>
            </div>
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
    </>
  );
}
