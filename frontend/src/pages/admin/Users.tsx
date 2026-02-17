import React, { useEffect, useState } from 'react';
import { Table, Tag, Typography } from 'antd';
import { api } from '../../api/client';
import type { User } from '../../api/client';

const roleColor = (role: string) => {
  switch (role) {
    case 'admin': return 'red';
    case 'instructor': return 'blue';
    default: return 'default';
  }
};

const Users: React.FC = () => {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.listUsers().then(setUsers).finally(() => setLoading(false));
  }, []);

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
          {
            title: 'Role',
            dataIndex: 'role',
            render: (v: string) => <Tag color={roleColor(v)}>{v}</Tag>,
          },
          {
            title: 'Active',
            dataIndex: 'active',
            render: (v: boolean) => v ? <Tag color="green">Active</Tag> : <Tag>Inactive</Tag>,
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
