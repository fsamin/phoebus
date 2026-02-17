import React, { useEffect, useState } from 'react';
import { Form, Input, Select, Button, Card, Typography, message, Breadcrumb } from 'antd';
import { useNavigate, useParams, Link } from 'react-router-dom';
import { api } from '../../api/client';
import type { RepoInput } from '../../api/client';

const RepoForm: React.FC = () => {
  const navigate = useNavigate();
  const { repoId } = useParams<{ repoId: string }>();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [authType, setAuthType] = useState('none');
  const isEdit = !!repoId;

  useEffect(() => {
    if (repoId) {
      api.getRepo(repoId).then((repo) => {
        form.setFieldsValue({
          clone_url: repo.clone_url,
          branch: repo.branch,
          auth_type: repo.auth_type,
        });
        setAuthType(repo.auth_type);
      });
    }
  }, [repoId, form]);

  const onFinish = async (values: RepoInput) => {
    setLoading(true);
    try {
      if (isEdit) {
        await api.updateRepo(repoId!, values);
        message.success('Repository updated, sync in progress');
      } else {
        await api.createRepo(values);
        message.success('Repository added, sync in progress');
      }
      navigate('/admin/repositories');
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ maxWidth: 600, margin: '0 auto' }}>
      <Breadcrumb items={[
        { title: <Link to="/admin/repositories">Repositories</Link> },
        { title: isEdit ? 'Edit' : 'Add' },
      ]} style={{ marginBottom: 16 }} />
      <Typography.Title level={3}>{isEdit ? 'Edit' : 'Add'} Repository</Typography.Title>
      <Card>
        <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ branch: 'main', auth_type: 'none' }}>
          <Form.Item name="clone_url" label="Clone URL" rules={[{ required: true }]}>
            <Input placeholder="https://github.com/org/repo.git" />
          </Form.Item>
          <Form.Item name="branch" label="Branch">
            <Input placeholder="main" />
          </Form.Item>
          <Form.Item name="auth_type" label="Authentication">
            <Select onChange={setAuthType}>
              <Select.Option value="none">None (public)</Select.Option>
              <Select.Option value="http-token">HTTP Token</Select.Option>
              <Select.Option value="http-basic">HTTP Basic</Select.Option>
              <Select.Option value="ssh-key">SSH Key</Select.Option>
            </Select>
          </Form.Item>
          {authType !== 'none' && (
            <Form.Item
              name="credentials"
              label={authType === 'ssh-key' ? 'SSH Private Key' : authType === 'http-token' ? 'Token' : 'username:password'}
              rules={[{ required: !isEdit }]}
              extra={isEdit ? 'Leave empty to keep existing credentials' : undefined}
            >
              {authType === 'ssh-key' ? (
                <Input.TextArea rows={4} placeholder={isEdit ? '••••••••' : '-----BEGIN OPENSSH PRIVATE KEY-----'} />
              ) : (
                <Input.Password placeholder={isEdit ? '••••••••' : authType === 'http-token' ? 'ghp_...' : 'user:password'} />
              )}
            </Form.Item>
          )}
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading}>
              Save & Sync
            </Button>
            <Button style={{ marginLeft: 8 }} onClick={() => navigate('/admin/repositories')}>
              Cancel
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default RepoForm;
