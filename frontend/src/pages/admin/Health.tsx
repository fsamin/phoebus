import React, { useEffect, useState, useCallback } from 'react';
import { Card, Typography, Tag, Row, Col, Spin, Statistic, Table } from 'antd';
import { CheckCircleOutlined, CloseCircleOutlined, SyncOutlined, ClockCircleOutlined } from '@ant-design/icons';

interface HealthData {
  api: { status: string; uptime: string };
  database: { connected: boolean };
  repositories: { total: number; synced: number; details: Array<{ id: string; clone_url: string; sync_status: string; sync_error?: string; last_synced_at?: string }> };
  active_users_24h: number;
  total_users: number;
}

const Health: React.FC = () => {
  const [data, setData] = useState<HealthData | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchHealth = useCallback(() => {
    fetch('/api/admin/health', { credentials: 'include' })
      .then((r) => r.json())
      .then(setData)
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    fetchHealth();
    const interval = setInterval(fetchHealth, 30_000);
    return () => clearInterval(interval);
  }, [fetchHealth]);

  if (loading || !data) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  const statusIcon = (ok: boolean) => ok
    ? <CheckCircleOutlined style={{ fontSize: 32, color: '#52c41a' }} />
    : <CloseCircleOutlined style={{ fontSize: 32, color: '#ff4d4f' }} />;

  return (
    <div>
      <Typography.Title level={3}>Platform Health</Typography.Title>
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={8}>
          <Card>
            <Card.Meta
              avatar={statusIcon(data.api.status === 'ok')}
              title="API"
              description={<><Tag color="green">Healthy</Tag> <Typography.Text type="secondary">Uptime: {data.api.uptime}</Typography.Text></>}
            />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card>
            <Card.Meta
              avatar={statusIcon(data.database.connected)}
              title="Database"
              description={<Tag color={data.database.connected ? 'green' : 'red'}>{data.database.connected ? 'Connected' : 'Disconnected'}</Tag>}
            />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card>
            <Card.Meta
              avatar={<SyncOutlined style={{ fontSize: 32, color: '#1890ff' }} />}
              title="Repositories"
              description={<>{data.repositories.synced}/{data.repositories.total} synced</>}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={12} sm={6}>
          <Card><Statistic title="Active Users (24h)" value={data.active_users_24h} prefix={<ClockCircleOutlined />} /></Card>
        </Col>
        <Col xs={12} sm={6}>
          <Card><Statistic title="Total Users" value={data.total_users} /></Card>
        </Col>
      </Row>

      {data.repositories.details.length > 0 && (
        <Card title="Repository Sync Status">
          <Table
            dataSource={data.repositories.details}
            rowKey="id"
            size="small"
            pagination={false}
            columns={[
              { title: 'Repository', dataIndex: 'clone_url', ellipsis: true },
              {
                title: 'Status',
                dataIndex: 'sync_status',
                width: 120,
                render: (v: string) => (
                  <Tag color={v === 'synced' ? 'green' : v === 'pending' ? 'orange' : v === 'failed' ? 'red' : 'blue'}>{v}</Tag>
                ),
              },
              {
                title: 'Last Synced',
                dataIndex: 'last_synced_at',
                render: (v?: string) => v ? new Date(v).toLocaleString() : '—',
              },
              {
                title: 'Error',
                dataIndex: 'sync_error',
                ellipsis: true,
                render: (v?: string) => v ? <Typography.Text type="danger">{v}</Typography.Text> : '—',
              },
            ]}
          />
        </Card>
      )}
    </div>
  );
};

export default Health;
