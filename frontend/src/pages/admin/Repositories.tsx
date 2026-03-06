import React, { useEffect, useState } from 'react';
import { Table, Button, Tag, Space, Typography, Popconfirm, Popover, message, Tooltip, Alert, Switch } from 'antd';
import { PlusOutlined, SyncOutlined, CopyOutlined, DeleteOutlined, EditOutlined, UnorderedListOutlined, KeyOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { api } from '../../api/client';
import type { GitRepository, RepoLearningPath, RepoOwner } from '../../api/client';
import { usePageTitle } from '../../hooks/usePageTitle';

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
  usePageTitle('Repositories');
  const navigate = useNavigate();
  const [repos, setRepos] = useState<Array<GitRepository & { path_titles: string[]; owners: RepoOwner[] }>>([]);
  const [loading, setLoading] = useState(true);
  const [sshPublicKey, setSSHPublicKey] = useState('');
  const [repoPaths, setRepoPaths] = useState<Record<string, RepoLearningPath[]>>({});
  const [pathsLoading, setPathsLoading] = useState<Record<string, boolean>>({});

  const fetchRepos = () => {
    setLoading(true);
    api.listRepos().then(setRepos).finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchRepos();
    api.sshPublicKey().then(r => setSSHPublicKey(r.public_key)).catch(() => {});
  }, []);

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

  const handleExpand = async (expanded: boolean, record: GitRepository) => {
    if (expanded && !repoPaths[record.id]) {
      setPathsLoading(prev => ({ ...prev, [record.id]: true }));
      try {
        const paths = await api.listRepoPaths(record.id);
        setRepoPaths(prev => ({ ...prev, [record.id]: paths }));
      } catch {
        message.error('Failed to load learning paths');
      } finally {
        setPathsLoading(prev => ({ ...prev, [record.id]: false }));
      }
    }
  };

  const handleTogglePath = async (repoId: string, pathId: string, enabled: boolean) => {
    try {
      await api.toggleRepoPath(repoId, pathId, enabled);
      message.success(enabled ? 'Learning path enabled' : 'Learning path disabled');
      setRepoPaths(prev => ({
        ...prev,
        [repoId]: prev[repoId].map(p => p.id === pathId ? { ...p, enabled } : p),
      }));
    } catch {
      message.error('Failed to update learning path');
    }
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

      {sshPublicKey && (
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
                Add this key as a read-only deploy key on your Git repositories to use SSH clone URLs (git@…).
              </Typography.Text>
            </div>
          }
        />
      )}

      <Table
        dataSource={repos}
        rowKey="id"
        loading={loading}
        expandable={{
          onExpand: handleExpand,
          expandedRowRender: (record: GitRepository) => {
            const paths = repoPaths[record.id];
            const isLoading = pathsLoading[record.id];
            if (isLoading) return <Typography.Text type="secondary">Loading…</Typography.Text>;
            if (!paths || paths.length === 0) return <Typography.Text type="secondary">No learning paths</Typography.Text>;
            return (
              <Table
                dataSource={paths}
                rowKey="id"
                pagination={false}
                size="small"
                columns={[
                  { title: 'Title', dataIndex: 'title' },
                  { title: 'Description', dataIndex: 'description', ellipsis: true },
                  {
                    title: 'Modules',
                    dataIndex: 'module_count',
                    width: 80,
                    align: 'center' as const,
                  },
                  {
                    title: 'Steps',
                    dataIndex: 'step_count',
                    width: 80,
                    align: 'center' as const,
                  },
                  {
                    title: 'Status',
                    dataIndex: 'enabled',
                    width: 120,
                    render: (enabled: boolean, path: RepoLearningPath) => (
                      <Space>
                        <Switch
                          checked={enabled}
                          onChange={(checked) => handleTogglePath(record.id, path.id, checked)}
                          size="small"
                        />
                        <Tag color={enabled ? 'green' : 'default'}>{enabled ? 'Active' : 'Disabled'}</Tag>
                      </Space>
                    ),
                  },
                ]}
              />
            );
          },
        }}
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
            title: 'Owners',
            dataIndex: 'owners',
            width: 180,
            render: (owners: RepoOwner[]) =>
              owners?.length > 0
                ? owners.map((o) => <Tag key={o.id}>{o.display_name}</Tag>)
                : <Typography.Text type="secondary">—</Typography.Text>,
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
