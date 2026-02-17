import React, { useEffect, useState } from 'react';
import { Typography, Card, Table, Spin, Tag, Breadcrumb, Row, Col, Statistic, Progress as AntProgress } from 'antd';
import { WarningOutlined } from '@ant-design/icons';
import { useParams, useNavigate, Link } from 'react-router-dom';

interface PathAnalytics {
  id: string;
  title: string;
  enrolled_count: number;
  completion_rate: number;
  steps: Array<{
    id: string;
    title: string;
    type: string;
    position: number;
    completion_rate: number;
    avg_attempts: number;
  }>;
  learners: Array<{
    user_id: string;
    username: string;
    display_name: string;
    completed: number;
    total: number;
    percentage: number;
  }>;
}

const PathAnalyticsView: React.FC = () => {
  const { pathId } = useParams<{ pathId: string }>();
  const navigate = useNavigate();
  const [data, setData] = useState<PathAnalytics | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch(`/api/analytics/paths/${pathId}`, { credentials: 'include' })
      .then((r) => r.json())
      .then(setData)
      .finally(() => setLoading(false));
  }, [pathId]);

  if (loading || !data) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  return (
    <div>
      <Breadcrumb items={[
        { title: <Link to="/analytics">Analytics</Link> },
        { title: data.title },
      ]} style={{ marginBottom: 16 }} />

      <Typography.Title level={2}>{data.title}</Typography.Title>

      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={8}>
          <Card><Statistic title="Enrolled Learners" value={data.enrolled_count} /></Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card><Statistic title="Completion Rate" value={data.completion_rate.toFixed(1)} suffix="%" /></Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card><Statistic title="Total Steps" value={data.steps.length} /></Card>
        </Col>
      </Row>

      <Card title="Step Performance" style={{ marginBottom: 24 }}>
        <Table
          dataSource={data.steps}
          rowKey="id"
          pagination={false}
          size="small"
          columns={[
            {
              title: 'Type',
              dataIndex: 'type',
              width: 120,
              render: (v: string) => <Tag>{v}</Tag>,
            },
            { title: 'Step', dataIndex: 'title' },
            {
              title: 'Completion',
              dataIndex: 'completion_rate',
              width: 150,
              render: (v: number) => (
                <span>
                  {v < 60 && <WarningOutlined style={{ color: '#faad14', marginRight: 4 }} />}
                  {v.toFixed(0)}%
                </span>
              ),
              sorter: (a, b) => a.completion_rate - b.completion_rate,
            },
            {
              title: 'Avg Attempts',
              dataIndex: 'avg_attempts',
              width: 120,
              render: (v: number) => v > 0 ? v.toFixed(1) : '—',
            },
          ]}
        />
      </Card>

      <Card title="Learners">
        <Table
          dataSource={data.learners}
          rowKey="user_id"
          pagination={false}
          size="small"
          onRow={(record) => ({ onClick: () => navigate(`/analytics/learners/${record.user_id}`), style: { cursor: 'pointer' } })}
          columns={[
            { title: 'Learner', dataIndex: 'display_name', render: (v: string, r) => v || r.username },
            {
              title: 'Progress',
              dataIndex: 'percentage',
              width: 200,
              render: (v: number) => <AntProgress percent={Math.round(v)} size="small" />,
            },
            {
              title: 'Completed',
              width: 120,
              render: (_: unknown, r) => `${r.completed}/${r.total}`,
            },
          ]}
        />
      </Card>
    </div>
  );
};

export default PathAnalyticsView;
