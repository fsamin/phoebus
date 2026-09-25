import React, { useEffect, useState } from 'react';
import { Table, Tag, Typography, Select, Switch, message, Input, Button, Modal, Form, Tooltip } from 'antd';
import { SearchOutlined, PlusOutlined, LockOutlined } from '@ant-design/icons';
import { api } from '../../api/client';
import type { User } from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
import { usePageTitle } from '../../hooks/usePageTitle';

const Users: React.FC = () => {
  usePageTitle('Users');
  const { user: currentUser } = useAuth();
  const [users, setUsers] = useState<Array<User & { completed_paths: number }>>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [createForm] = Form.useForm();

  // Debounce search 300ms; a new search starts back on the first page.
  // Both updates are batched into a single render, hence a single fetch.
  // An unchanged search must not reset the page (e.g. right after mount).
  useEffect(() => {
    const t = setTimeout(() => {
      const next = searchInput.trim();
      if (next === search) return;
      setSearch(next);
      setPage(1);
    }, 300);
    return () => clearTimeout(t);
  }, [searchInput, search]);

  const loadUsers = () => {
    setLoading(true);
    api.listUsers(page, 20, search).then((data) => {
      setUsers(data.users);
      setTotal(data.total);
    }).finally(() => setLoading(false));
  };

  useEffect(() => { loadUsers(); }, [page, search]);

  const updateUser = async (userId: string, patch: { role?: string; active?: boolean }) => {
    const resp = await fetch(`/api/admin/users/${userId}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(patch),
    });
    if (!resp.ok) {
      const data = await resp.json();
      message.error(data.error || 'Update failed');
      return;
    }
    message.success('User updated');
    loadUsers();
  };

  const handleCreateUser = async (values: { username: string; display_name: string; email?: string; role: string; password: string }) => {
    setCreateLoading(true);
    try {
      await api.createUser(values);
      message.success('User created');
      setCreateModalOpen(false);
      createForm.resetFields();
      loadUsers();
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setCreateLoading(false);
    }
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={3} style={{ margin: 0 }}>Users</Typography.Title>
        <div style={{ display: 'flex', gap: 12 }}>
          <Input
            prefix={<SearchOutlined />}
            placeholder="Search users..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            allowClear
            style={{ width: 300 }}
          />
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalOpen(true)}>
            Add User
          </Button>
        </div>
      </div>
      <Table
        dataSource={users}
        rowKey="id"
        loading={loading}
        rowClassName={(record: User) => record.active ? '' : 'deactivated-row'}
        pagination={{
          current: page,
          pageSize: 20,
          total,
          onChange: (p) => setPage(p),
          showSizeChanger: false,
          showTotal: (t) => `${t} users`,
        }}
        columns={[
          { title: 'Username', dataIndex: 'username' },
          { title: 'Display Name', dataIndex: 'display_name' },
          { title: 'Email', dataIndex: 'email', render: (v?: string) => v || '—' },
          { title: 'Provider', dataIndex: 'auth_provider', width: 90 },
          {
            title: 'Role',
            dataIndex: 'role',
            width: 140,
            render: (v: string, record: User) => record.role_locked ? (
              <Tooltip title="Role managed by configuration">
                <span><Tag color="red" icon={<LockOutlined />}>{v}</Tag></span>
              </Tooltip>
            ) : (
              <Select
                value={v}
                size="small"
                style={{ width: 120 }}
                onChange={(role) => updateUser(record.id, { role })}
                options={[
                  { value: 'learner', label: <Tag>learner</Tag> },
                  { value: 'instructor', label: <Tag color="blue">instructor</Tag> },
                  { value: 'admin', label: <Tag color="red">admin</Tag> },
                ]}
              />
            ),
          },
          {
            title: 'Active',
            dataIndex: 'active',
            width: 80,
            render: (v: boolean, record: User) => (
              <Switch
                checked={v}
                size="small"
                disabled={record.id === currentUser?.id}
                onChange={(active) => updateUser(record.id, { active })}
              />
            ),
          },
          {
            title: 'Completed Paths',
            dataIndex: 'completed_paths',
            width: 130,
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            sorter: (a: any, b: any) => (a.completed_paths ?? 0) - (b.completed_paths ?? 0),
          },
          {
            title: 'Last Login',
            dataIndex: 'last_login_at',
            render: (v?: string) => v ? new Date(v).toLocaleString() : '—',
          },
        ]}
      />

      <Modal
        title="Create Local User"
        open={createModalOpen}
        onCancel={() => { setCreateModalOpen(false); createForm.resetFields(); }}
        footer={null}
      >
        <Form form={createForm} layout="vertical" onFinish={handleCreateUser} initialValues={{ role: 'learner' }}>
          <Form.Item name="username" label="Username" rules={[{ required: true, min: 4, max: 32 }]}>
            <Input />
          </Form.Item>
          <Form.Item name="display_name" label="Display Name" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="email" label="Email">
            <Input type="email" />
          </Form.Item>
          <Form.Item name="role" label="Role">
            <Select options={[
              { value: 'learner', label: 'Learner' },
              { value: 'instructor', label: 'Instructor' },
              { value: 'admin', label: 'Admin' },
            ]} />
          </Form.Item>
          <Form.Item name="password" label="Password" rules={[{ required: true, min: 8 }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={createLoading} block>
              Create User
            </Button>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Users;
