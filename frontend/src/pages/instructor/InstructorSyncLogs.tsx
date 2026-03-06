import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Table, Tag, Typography, Tooltip, Button, Space, Drawer, Spin, Empty, message } from 'antd';
import { ArrowLeftOutlined, SyncOutlined, FileTextOutlined } from '@ant-design/icons';
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

const levelStyles: Record<string, { color: string; bg: string }> = {
  debug: { color: '#8c8c8c', bg: 'transparent' },
  info:  { color: '#1677ff', bg: 'transparent' },
  warn:  { color: '#fa8c16', bg: 'rgba(250,140,22,0.06)' },
  error: { color: '#ff4d4f', bg: 'rgba(255,77,79,0.06)' },
};

function formatFieldValue(v: unknown): string {
  if (v === null || v === undefined) return '';
  if (typeof v === 'object') return JSON.stringify(v);
  return String(v);
}

function LogLine({ entry }: { entry: SyncJobLogEntry }) {
  const style = levelStyles[entry.level] || levelStyles.info;
  const fields = entry.fields
    ? Object.entries(entry.fields).filter(([k]) => !['repo_id', 'job_id'].includes(k))
    : [];
  const ts = new Date(entry.timestamp).toLocaleTimeString(undefined, {
    hour12: false,
    fractionalSecondDigits: 3,
  } as Intl.DateTimeFormatOptions);

  return (
    <div
      style={{
        display: 'flex',
        gap: 8,
        padding: '3px 8px',
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
        fontSize: 12,
        lineHeight: '20px',
        backgroundColor: style.bg,
        borderLeft: `3px solid ${style.color}`,
        alignItems: 'baseline',
      }}
    >
      <span style={{ color: '#8c8c8c', flexShrink: 0 }}>{ts}</span>
      <span
        style={{
          color: style.color,
          fontWeight: 600,
          width: 40,
          textAlign: 'right',
          flexShrink: 0,
          textTransform: 'uppercase',
        }}
      >
        {entry.level}
      </span>
      <span style={{ color: '#262626', flexGrow: 1, wordBreak: 'break-word' }}>
        {entry.message}
        {fields.length > 0 && (
          <span style={{ color: '#8c8c8c', marginLeft: 8 }}>
            {fields.map(([k, v]) => (
              <span key={k} style={{ marginRight: 8 }}>
                <span style={{ color: '#595959' }}>{k}</span>
                <span style={{ color: '#8c8c8c' }}>=</span>
                <span style={{ color: '#1677ff' }}>{formatFieldValue(v)}</span>
              </span>
            ))}
          </span>
        )}
      </span>
    </div>
  );
}

export default function InstructorSyncLogs() {
  const { repoId } = useParams<{ repoId: string }>();
  const [logs, setLogs] = useState<SyncLog[]>([]);
  const [repo, setRepo] = useState<GitRepository | null>(null);
  const [loading, setLoading] = useState(true);
  const [drawerJob, setDrawerJob] = useState<SyncLog | null>(null);
  const [jobLogs, setJobLogs] = useState<Record<string, SyncJobLogEntry[]>>({});
  const [jobLogsLoading, setJobLogsLoading] = useState<Record<string, boolean>>({});

  const fetchData = () => {
    if (!repoId) return;
    setLoading(true);
    Promise.all([
      api.instructorSyncLogs(repoId),
      api.instructorListRepos(),
    ]).then(([logsData, repos]) => {
      setLogs(logsData);
      const found = repos.find((r) => r.id === repoId);
      if (found) setRepo(found);
    }).finally(() => setLoading(false));
  };

  useEffect(() => { fetchData(); }, [repoId]);

  const handleSync = async () => {
    if (!repoId) return;
    try {
      await api.instructorSyncRepo(repoId);
      message.success('Sync triggered');
      fetchData();
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const openDrawer = async (record: SyncLog) => {
    setDrawerJob(record);
    if (jobLogs[record.id] || !repoId) return;
    setJobLogsLoading((prev) => ({ ...prev, [record.id]: true }));
    try {
      const entries = await api.instructorSyncJobLogs(repoId, record.id);
      setJobLogs((prev) => ({ ...prev, [record.id]: entries }));
    } catch {
      setJobLogs((prev) => ({ ...prev, [record.id]: [] }));
    } finally {
      setJobLogsLoading((prev) => ({ ...prev, [record.id]: false }));
    }
  };

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
    {
      title: '',
      key: 'actions',
      width: 50,
      render: (_v: unknown, record: SyncLog) => (
        <Button
          type="text"
          size="small"
          icon={<FileTextOutlined />}
          onClick={() => openDrawer(record)}
        />
      ),
    },
  ];

  const drawerEntries = drawerJob ? jobLogs[drawerJob.id] : undefined;
  const drawerLoading = drawerJob ? jobLogsLoading[drawerJob.id] : false;
  const drawerTitle = drawerJob
    ? `Sync Job — ${new Date(drawerJob.started_at || drawerJob.created_at).toLocaleString()}`
    : 'Sync Job Logs';

  return (
    <div style={{ padding: 24 }}>
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Link to="/">
          <Button type="link" icon={<ArrowLeftOutlined />} style={{ padding: 0 }}>
            Back to Dashboard
          </Button>
        </Link>

        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Title level={4} style={{ margin: 0 }}>
            Sync Logs{repo ? `: ${repo.clone_url} (${repo.branch})` : ''}
          </Title>
          <Button icon={<SyncOutlined />} onClick={handleSync}>
            Sync Now
          </Button>
        </div>

        <Table
          dataSource={logs}
          columns={columns}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 20, showSizeChanger: false }}
          size="middle"
          locale={{ emptyText: 'No sync jobs found for this repository' }}
          onRow={(record) => ({
            onClick: () => openDrawer(record),
            style: { cursor: 'pointer' },
          })}
        />
      </Space>

      <Drawer
        title={drawerTitle}
        placement="right"
        width={720}
        open={!!drawerJob}
        onClose={() => setDrawerJob(null)}
        extra={
          drawerJob && (
            <Space>
              <Tag color={statusColors[drawerJob.status] || 'default'}>{drawerJob.status}</Tag>
              <Text type="secondary">{formatDuration(drawerJob.started_at, drawerJob.completed_at)}</Text>
            </Space>
          )
        }
      >
        {drawerJob?.error && (
          <div
            style={{
              padding: '8px 12px',
              marginBottom: 12,
              backgroundColor: 'rgba(255,77,79,0.06)',
              border: '1px solid #ffccc7',
              borderRadius: 6,
              fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
              fontSize: 12,
              color: '#cf1322',
              wordBreak: 'break-word',
            }}
          >
            {drawerJob.error}
          </div>
        )}

        {drawerLoading && (
          <div style={{ textAlign: 'center', padding: 48 }}>
            <Spin tip="Loading logs..." />
          </div>
        )}

        {!drawerLoading && (!drawerEntries || drawerEntries.length === 0) && (
          <Empty description="No detailed logs available" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        )}

        {!drawerLoading && drawerEntries && drawerEntries.length > 0 && (
          <div
            style={{
              backgroundColor: '#fafafa',
              border: '1px solid #f0f0f0',
              borderRadius: 6,
              overflow: 'auto',
              maxHeight: 'calc(100vh - 200px)',
            }}
          >
            {drawerEntries.map((entry, i) => (
              <LogLine key={i} entry={entry} />
            ))}
          </div>
        )}
      </Drawer>
    </div>
  );
}
