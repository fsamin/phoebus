import React, { useEffect, useState } from 'react';
import { Table, Tag, Typography, Select, Switch, message, Input } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { api } from '../../api/client';
import type { User } from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';

const Users: React.FC = () => {
  const { user: currentUser } = useAuth();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');

  // Debounce search 300ms
  useEffect(() => {
    const t = setTimeout(() => setSearch(searchInput), 300);
    return () => clearTimeout(t);
  }, [searchInput]);

  const loadUsers = (p = page) => {
    setLoading(true);
    api.listUsers(p).then((data) => {
      setUsers(data.users);
      setTotal(data.total);
    }).finally(() => setLoading(false));
  };

  useEffect(() => { loadUsers(); }, [page]);

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

  const filteredUsers = search
    ? users.filter((u) =>
        u.username.toLowerCase().includes(search.toLowerCase()) ||
        (u.display_name || '').toLowerCase().includes(search.toLowerCase()) ||
        (u.email || '').toLowerCase().includes(search.toLowerCase()))
    : users;

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={3} style={{ margin: 0 }}>Users</Typography.Title>
        <Input
          prefix={<SearchOutlined />}
          placeholder="Search users..."
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          allowClear
          style={{ width: 300 }}
        />
      </div>
      <Table
        dataSource={filteredUsers}
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
