import React, { useEffect, useState } from 'react';
import { Typography, Card, Row, Col, Table, Spin, Statistic } from 'antd';
import { TeamOutlined, BookOutlined, TrophyOutlined, ExperimentOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';

interface AnalyticsOverviewData {
  total_paths: number;
  total_learners: number;
  completion_rate: number;
  total_attempts: number;
  paths: Array<{
    id: string;
    title: string;
    enrolled_count: number;
    completion_rate: number;
    total_steps: number;
  }>;
}

interface ActivityEvent {
  user_id: string;
  username: string;
  display_name: string;
  step_title: string;
  path_title: string;
  event: string;
  created_at: string;
}

const Analytics: React.FC = () => {
  const navigate = useNavigate();
  const [overview, setOverview] = useState<AnalyticsOverviewData | null>(null);
  const [activity, setActivity] = useState<ActivityEvent[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      fetch('/api/analytics/overview', { credentials: 'include' }).then((r) => r.json()),
      fetch('/api/analytics/activity?limit=10', { credentials: 'include' }).then((r) => r.json()),
    ])
      .then(([o, a]) => { setOverview(o); setActivity(a); })
      .finally(() => setLoading(false));
  }, []);

  if (loading || !overview) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  return (
    <div>
      <Typography.Title level={2}>Analytics</Typography.Title>

      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={6}>
          <Card><Statistic title="Learning Paths" value={overview.total_paths} prefix={<BookOutlined />} /></Card>
        </Col>
        <Col xs={24} sm={6}>
          <Card><Statistic title="Enrolled Learners" value={overview.total_learners} prefix={<TeamOutlined />} /></Card>
        </Col>
        <Col xs={24} sm={6}>
          <Card><Statistic title="Completion Rate" value={overview.completion_rate.toFixed(1)} suffix="%" prefix={<TrophyOutlined />} /></Card>
        </Col>
        <Col xs={24} sm={6}>
          <Card><Statistic title="Total Attempts" value={overview.total_attempts} prefix={<ExperimentOutlined />} /></Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={16}>
          <Card title="Learning Paths">
            <Table
              dataSource={overview.paths}
              rowKey="id"
              onRow={(record) => ({ onClick: () => navigate(`/analytics/paths/${record.id}`), style: { cursor: 'pointer' } })}
              columns={[
                { title: 'Title', dataIndex: 'title' },
                { title: 'Enrolled', dataIndex: 'enrolled_count', width: 100, sorter: (a, b) => a.enrolled_count - b.enrolled_count },
                { title: 'Steps', dataIndex: 'total_steps', width: 80 },
                { title: 'Completion', dataIndex: 'completion_rate', width: 120, render: (v: number) => `${v.toFixed(1)}%`, sorter: (a, b) => a.completion_rate - b.completion_rate },
              ]}
              pagination={false}
              size="small"
            />
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card title="Recent Activity">
            {activity.map((e, i) => (
              <div key={i} style={{ marginBottom: 12, paddingBottom: 12, borderBottom: '1px solid #f0f0f0' }}>
                <Typography.Text
                  strong
                  style={{ cursor: 'pointer', color: '#1890ff' }}
                  onClick={() => navigate(`/analytics/learners/${e.user_id}`)}
                >
                  {e.display_name || e.username}
                </Typography.Text>
                <Typography.Text> {e.event === 'completed' ? '✅ completed' : '▶ started'} </Typography.Text>
                <Typography.Text>{e.step_title}</Typography.Text>
                <br />
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  {e.path_title} · {new Date(e.created_at).toLocaleString()}
                </Typography.Text>
              </div>
            ))}
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Analytics;
