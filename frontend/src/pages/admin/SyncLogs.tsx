import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Table, Tag, Typography, Tooltip, Button, Space } from 'antd';
import { ArrowLeftOutlined, SyncOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { api } from '../../api/client';
import type { SyncLog, GitRepository } from '../../api/client';

const { Title, Text } = Typography;

function formatDuration(startedAt: string | null, completedAt: string | null): string {
  if (!startedAt || !completedAt) return '—';
  const ms = new Date(completedAt).getTime() - new Date(startedAt).getTime();
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const mins = Math.floor(ms / 60000);
  const secs = Math.round((ms % 60000) / 1000);
  return `${mins}m ${secs}s`;
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

const statusColors: Record<string, string> = {
  done: 'green',
  failed: 'red',
  processing: 'blue',
  pending: 'default',
};

export default function SyncLogs() {
  const { repoId } = useParams<{ repoId: string }>();
  const [logs, setLogs] = useState<SyncLog[]>([]);
  const [repo, setRepo] = useState<GitRepository | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!repoId) return;
    Promise.all([
      api.syncLogs(repoId),
      api.listRepos(),
    ]).then(([logsData, repos]) => {
      setLogs(logsData);
      const found = repos.find((r) => r.id === repoId);
      if (found) setRepo(found);
    }).finally(() => setLoading(false));
  }, [repoId]);

  const columns: ColumnsType<SyncLog> = [
    {
      title: '#',
      key: 'index',
      width: 60,
      render: (_v, _r, i) => logs.length - i,
    },
    {
      title: 'Status',
      dataIndex: 'status',
      width: 120,
      render: (status: string) => (
        <Tag
          color={statusColors[status] || 'default'}
          icon={status === 'processing' ? <SyncOutlined spin /> : undefined}
        >
          {status}
        </Tag>
      ),
    },
    {
      title: 'Started',
      dataIndex: 'started_at',
      width: 200,
      render: (v: string | null, record: SyncLog) => {
        const ts = v || record.created_at;
        return (
          <Tooltip title={new Date(ts).toLocaleString()}>
            <span>{relativeTime(ts)}</span>
          </Tooltip>
        );
      },
    },
    {
      title: 'Duration',
      key: 'duration',
      width: 120,
      render: (_v: unknown, record: SyncLog) => formatDuration(record.started_at, record.completed_at),
    },
    {
      title: 'Attempts',
      dataIndex: 'attempts',
      width: 90,
      align: 'center' as const,
    },
    {
      title: 'Error',
      dataIndex: 'error',
      ellipsis: true,
      render: (error: string | null) =>
        error ? (
          <Tooltip title={error} overlayStyle={{ maxWidth: 500 }}>
            <Text type="danger" style={{ cursor: 'pointer' }}>
              {error.length > 80 ? error.slice(0, 80) + '…' : error}
            </Text>
          </Tooltip>
        ) : (
          <Text type="secondary">—</Text>
        ),
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Link to="/admin/repositories">
          <Button type="link" icon={<ArrowLeftOutlined />} style={{ padding: 0 }}>
            Back to Repositories
          </Button>
        </Link>

        <Title level={4} style={{ margin: 0 }}>
          Sync Logs{repo ? `: ${repo.clone_url} (${repo.branch})` : ''}
        </Title>

        <Table
          dataSource={logs}
          columns={columns}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 20, showSizeChanger: false }}
          size="middle"
          locale={{ emptyText: 'No sync jobs found for this repository' }}
        />
      </Space>
    </div>
  );
}
