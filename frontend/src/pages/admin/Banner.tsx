import React, { useEffect, useState } from 'react';
import { Form, Input, Select, Switch, Button, Card, Typography, message } from 'antd';
import { api } from '../../api/client';
import type { Banner as BannerSettings } from '../../api/client';
import { usePageTitle } from '../../hooks/usePageTitle';
import DashboardBanner from '../../components/DashboardBanner';

const MAX_LENGTH = 2000;

const Banner: React.FC = () => {
  usePageTitle('Banner');
  const [form] = Form.useForm<BannerSettings>();
  const [loading, setLoading] = useState(false);

  // No early return on missing data: "never configured" is a legitimate state,
  // the form renders straight away with its defaults and the fetch fills it in.
  useEffect(() => {
    api.getBanner().then((banner) => form.setFieldsValue(banner)).catch(() => {});
  }, [form]);

  const onFinish = async (values: BannerSettings) => {
    setLoading(true);
    try {
      const saved = await api.updateBanner(values);
      form.setFieldsValue(saved);
      message.success(saved.enabled ? 'Banner published' : 'Banner disabled');
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const level = Form.useWatch('level', form);
  const messageMD = Form.useWatch('message_md', form);

  return (
    <div style={{ maxWidth: 600, margin: '0 auto' }}>
      <Typography.Title level={3}>Dashboard Banner</Typography.Title>
      <Typography.Paragraph type="secondary">
        Shown at the top of the dashboard to every user. Useful for maintenance notices or
        announcements.
      </Typography.Paragraph>

      <Card>
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
          initialValues={{ enabled: false, level: 'info', message_md: '' }}
        >
          <Form.Item name="enabled" label="Enabled" valuePropName="checked">
            <Switch />
          </Form.Item>

          <Form.Item name="level" label="Level">
            <Select
              options={[
                { value: 'info', label: 'Info' },
                { value: 'warning', label: 'Warning' },
                { value: 'danger', label: 'Danger' },
              ]}
            />
          </Form.Item>

          <Form.Item
            name="message_md"
            label="Message"
            rules={[{ max: MAX_LENGTH }]}
            extra="Markdown is supported. Avoid headings, they break the banner layout, and external images, which the content security policy blocks."
          >
            <Input.TextArea rows={6} showCount maxLength={MAX_LENGTH} />
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading}>
              Save
            </Button>
          </Form.Item>
        </Form>
      </Card>

      {messageMD ? (
        <>
          <Typography.Title level={5} style={{ marginTop: 24 }}>
            Preview
          </Typography.Title>
          <DashboardBanner level={level || 'info'} message_md={messageMD} />
        </>
      ) : null}
    </div>
  );
};

export default Banner;
