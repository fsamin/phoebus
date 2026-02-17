import React, { useEffect, useState } from 'react';
import { Table, Button, Tag, Space, Typography, Popconfirm, Popover, message, Tooltip } from 'antd';
import { PlusOutlined, SyncOutlined, CopyOutlined, DeleteOutlined, EditOutlined, UnorderedListOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { api } from '../../api/client';
import type { GitRepository } from '../../api/client';

function relativeTime(iso?: string): string {
  if (!iso) return '—';
  const diff = Date.now() - new Date(iso).getTime();
  const hours = Math.floor(diff / 3600000);
  if (hours < 1) return 'Just now';
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return days === 1 ? 'Yesterday' : `${days}d ago`;
}

const Repositories: React.FC = () => {
  const navigate = useNavigate();
  const [repos, setRepos] = useState<Array<GitRepository & { path_titles: string[] }>>([]);
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
            title: 'Name',
            dataIndex: 'path_titles',
            render: (v: string[]) => v.length > 0 ? v.join(', ') : '—',
          },
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
            render: (v: string, r: GitRepository) => {
              const tag = (
                <Tag color={statusColor(v)} icon={v === 'syncing' ? <SyncOutlined spin /> : undefined}>
                  {v}
                </Tag>
              );
              return v === 'error' && r.sync_error ? (
                <Popover content={<Typography.Text type="danger">{r.sync_error}</Typography.Text>} title="Sync Error" trigger="click">
                  <span style={{ cursor: 'pointer' }}>{tag}</span>
                </Popover>
              ) : tag;
            },
          },
          {
            title: 'Last Synced',
            dataIndex: 'last_synced_at',
            width: 180,
            render: (v?: string) => (
              <Tooltip title={v ? new Date(v).toLocaleString() : undefined}>
                {relativeTime(v)}
              </Tooltip>
            ),
          },
          {
            title: 'Actions',
            width: 200,
            render: (_: unknown, r: GitRepository) => (
              <Space>
                <Button size="small" icon={<EditOutlined />} onClick={() => navigate(`/admin/repositories/${r.id}/edit`)} />
                <Button size="small" icon={<SyncOutlined />} onClick={() => handleSync(r.id)} />
                <Tooltip title="Sync Logs">
                  <Button size="small" icon={<UnorderedListOutlined />} onClick={() => navigate(`/admin/repositories/${r.id}/sync-logs`)} />
                </Tooltip>
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
