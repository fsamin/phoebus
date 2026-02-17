import React, { useEffect, useState } from 'react';
import { Table, Tag, Typography, Select, Switch, message } from 'antd';
import { api } from '../../api/client';
import type { User } from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';

const Users: React.FC = () => {
  const { user: currentUser } = useAuth();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);

  const loadUsers = () => {
    setLoading(true);
    api.listUsers().then(setUsers).finally(() => setLoading(false));
  };

  useEffect(() => { loadUsers(); }, []);

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

  return (
    <div>
      <Typography.Title level={3}>Users</Typography.Title>
      <Table
        dataSource={users}
        rowKey="id"
        loading={loading}
        columns={[
          { title: 'Username', dataIndex: 'username' },
          { title: 'Display Name', dataIndex: 'display_name' },
          { title: 'Email', dataIndex: 'email', render: (v?: string) => v || '—' },
          { title: 'Provider', dataIndex: 'auth_provider', width: 90 },
          {
            title: 'Role',
            dataIndex: 'role',
            width: 140,
            render: (v: string, record: User) => (
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
            title: 'Last Login',
            dataIndex: 'last_login_at',
            render: (v?: string) => v ? new Date(v).toLocaleString() : '—',
          },
        ]}
      />
    </div>
  );
};

export default Users;
