import React, { useEffect, useState } from 'react';
import { Table, Button, Tag, Space, Typography, Popconfirm, message, Tooltip } from 'antd';
import { PlusOutlined, SyncOutlined, CopyOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { api } from '../../api/client';
import type { GitRepository } from '../../api/client';

const Repositories: React.FC = () => {
  const navigate = useNavigate();
  const [repos, setRepos] = useState<GitRepository[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchRepos = () => {
    setLoading(true);
    api.listRepos().then(setRepos).finally(() => setLoading(false));
  };

  useEffect(fetchRepos, []);

  const handleSync = async (id: string) => {
    await api.syncRepo(id);
    message.success('Sync triggered');
    fetchRepos();
  };

  const handleDelete = async (id: string) => {
    await api.deleteRepo(id);
    message.success('Repository deleted');
    fetchRepos();
  };

  const handleCopyWebhook = (uuid: string) => {
    navigator.clipboard.writeText(`${window.location.origin}/api/webhooks/${uuid}`);
    message.success('Webhook URL copied');
  };

  const statusColor = (s: string) => {
    switch (s) {
      case 'synced': return 'green';
      case 'syncing': return 'blue';
      case 'error': return 'red';
      default: return 'default';
    }
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={3}>Repositories</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/admin/repositories/new')}>
          Add Repository
        </Button>
      </div>

      <Table
        dataSource={repos}
        rowKey="id"
        loading={loading}
        columns={[
          {
            title: 'Clone URL',
            dataIndex: 'clone_url',
            ellipsis: true,
          },
          {
            title: 'Branch',
            dataIndex: 'branch',
            width: 100,
          },
          {
            title: 'Auth',
            dataIndex: 'auth_type',
            width: 100,
            render: (v: string) => <Tag>{v}</Tag>,
          },
          {
            title: 'Status',
            dataIndex: 'sync_status',
            width: 120,
            render: (v: string, r: GitRepository) => (
              <Tooltip title={r.sync_error}>
                <Tag color={statusColor(v)} icon={v === 'syncing' ? <SyncOutlined spin /> : undefined}>
                  {v}
                </Tag>
              </Tooltip>
            ),
          },
          {
            title: 'Last Synced',
            dataIndex: 'last_synced_at',
            width: 180,
            render: (v?: string) => v ? new Date(v).toLocaleString() : '—',
          },
          {
            title: 'Actions',
            width: 200,
            render: (_: unknown, r: GitRepository) => (
              <Space>
                <Button size="small" icon={<EditOutlined />} onClick={() => navigate(`/admin/repositories/${r.id}/edit`)} />
                <Button size="small" icon={<SyncOutlined />} onClick={() => handleSync(r.id)} />
                <Button size="small" icon={<CopyOutlined />} onClick={() => handleCopyWebhook(r.webhook_uuid)} />
                <Popconfirm title="Delete this repository?" onConfirm={() => handleDelete(r.id)}>
                  <Button size="small" danger icon={<DeleteOutlined />} />
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />
    </div>
  );
};

export default Repositories;
