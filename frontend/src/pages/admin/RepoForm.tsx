import React, { useEffect, useState } from 'react';
import { Form, Input, Select, Button, Card, Typography, message, Breadcrumb, Alert } from 'antd';
import { KeyOutlined } from '@ant-design/icons';
import { useNavigate, useParams, Link } from 'react-router-dom';
import { api } from '../../api/client';
import type { RepoInput, RepoOwner } from '../../api/client';
import { usePageTitle } from '../../hooks/usePageTitle';

const RepoForm: React.FC = () => {
  const { repoId } = useParams<{ repoId: string }>();
  usePageTitle(repoId ? 'Edit Repository' : 'Add Repository');
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [authType, setAuthType] = useState('none');
  const [sshPublicKey, setSSHPublicKey] = useState('');
  const [instructorUsers, setInstructorUsers] = useState<RepoOwner[]>([]);
  const isEdit = !!repoId;

  useEffect(() => {
    api.sshPublicKey().then(r => setSSHPublicKey(r.public_key)).catch(() => {});
    api.listInstructorUsers().then(setInstructorUsers).catch(() => {});
    if (repoId) {
      api.getRepo(repoId).then((repo) => {
        form.setFieldsValue({
          clone_url: repo.clone_url,
          branch: repo.branch,
          auth_type: repo.auth_type,
          owner_ids: repo.owners?.map((o) => o.id) || [],
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
              <Select.Option value="instance-ssh-key">Instance SSH Key</Select.Option>
              <Select.Option value="http-token">HTTP Token</Select.Option>
              <Select.Option value="http-basic">HTTP Basic</Select.Option>
            </Select>
          </Form.Item>
          {authType === 'instance-ssh-key' && sshPublicKey && (
            <Alert
              style={{ marginBottom: 16 }}
              type="info"
              showIcon
              icon={<KeyOutlined />}
              message="Instance SSH Public Key"
              description={
                <div>
                  <Typography.Text code copyable style={{ wordBreak: 'break-all' }}>{sshPublicKey}</Typography.Text>
                  <br />
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    Add this key as a read-only deploy key on your Git repository.
                  </Typography.Text>
                </div>
              }
            />
          )}
          {authType !== 'none' && authType !== 'instance-ssh-key' && (
            <Form.Item
              name="credentials"
              label={authType === 'http-token' ? 'Token' : 'username:password'}
              rules={[{ required: !isEdit }]}
              extra={isEdit ? 'Leave empty to keep existing credentials' : undefined}
            >
              <Input.Password placeholder={isEdit ? '••••••••' : authType === 'http-token' ? 'ghp_...' : 'user:password'} />
            </Form.Item>
          )}
          <Form.Item name="owner_ids" label="Owners (Instructors)">
            <Select
              mode="multiple"
              placeholder="Select instructors..."
              options={instructorUsers.map((u) => ({
                label: `${u.display_name} (${u.username})`,
                value: u.id,
              }))}
              filterOption={(input, option) =>
                (option?.label as string)?.toLowerCase().includes(input.toLowerCase())
              }
              allowClear
            />
          </Form.Item>
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
