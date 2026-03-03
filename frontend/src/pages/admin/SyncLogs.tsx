import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Table, Tag, Typography, Tooltip, Button, Space } from 'antd';
import { ArrowLeftOutlined, SyncOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { api } from '../../api/client';
import type { SyncLog, SyncJobLogEntry, GitRepository } from '../../api/client';

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

const levelColors: Record<string, string> = {
  debug: 'default',
  info: 'blue',
  warn: 'orange',
  error: 'red',
};

export default function SyncLogs() {
  const { repoId } = useParams<{ repoId: string }>();
  const [logs, setLogs] = useState<SyncLog[]>([]);
  const [repo, setRepo] = useState<GitRepository | null>(null);
  const [loading, setLoading] = useState(true);
  const [jobLogs, setJobLogs] = useState<Record<string, SyncJobLogEntry[]>>({});
  const [jobLogsLoading, setJobLogsLoading] = useState<Record<string, boolean>>({});

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

  const loadJobLogs = async (jobId: string) => {
    if (jobLogs[jobId] || !repoId) return;
    setJobLogsLoading((prev) => ({ ...prev, [jobId]: true }));
    try {
      const entries = await api.syncJobLogs(repoId, jobId);
      setJobLogs((prev) => ({ ...prev, [jobId]: entries }));
    } catch {
      setJobLogs((prev) => ({ ...prev, [jobId]: [] }));
    } finally {
      setJobLogsLoading((prev) => ({ ...prev, [jobId]: false }));
    }
  };

  const logEntryColumns: ColumnsType<SyncJobLogEntry> = [
    {
      title: 'Time',
      dataIndex: 'timestamp',
      width: 180,
      render: (v: string) => new Date(v).toLocaleTimeString(undefined, { hour12: false, fractionalSecondDigits: 3 } as Intl.DateTimeFormatOptions),
    },
    {
      title: 'Level',
      dataIndex: 'level',
      width: 80,
      render: (level: string) => <Tag color={levelColors[level] || 'default'}>{level}</Tag>,
    },
    {
      title: 'Message',
      dataIndex: 'message',
      ellipsis: true,
    },
    {
      title: 'Details',
      dataIndex: 'fields',
      width: 300,
      ellipsis: true,
      render: (fields: Record<string, unknown> | undefined) => {
        if (!fields || Object.keys(fields).length === 0) return <Text type="secondary">—</Text>;
        // Filter out repo_id and job_id (already shown in parent row)
        const filtered = Object.entries(fields).filter(([k]) => !['repo_id', 'job_id'].includes(k));
        if (filtered.length === 0) return <Text type="secondary">—</Text>;
        const text = filtered.map(([k, v]) => `${k}=${v}`).join(' ');
        return (
          <Tooltip title={text} overlayStyle={{ maxWidth: 600 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>{text.length > 50 ? text.slice(0, 50) + '…' : text}</Text>
          </Tooltip>
        );
      },
    },
  ];

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
          expandable={{
            expandedRowRender: (record) => {
              const entries = jobLogs[record.id];
              if (jobLogsLoading[record.id]) {
                return <Text type="secondary">Loading logs...</Text>;
              }
              if (!entries || entries.length === 0) {
                return <Text type="secondary">No detailed logs available</Text>;
              }
              return (
                <Table
                  dataSource={entries}
                  columns={logEntryColumns}
                  rowKey={(_, i) => String(i)}
                  pagination={false}
                  size="small"
                  style={{ margin: 0 }}
                />
              );
            },
            onExpand: (expanded, record) => {
              if (expanded) loadJobLogs(record.id);
            },
          }}
        />
      </Space>
    </div>
  );
}
