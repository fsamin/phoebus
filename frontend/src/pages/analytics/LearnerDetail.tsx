import React, { useEffect, useState } from 'react';
import { Typography, Card, Table, Spin, Tag, Breadcrumb, Row, Col, Progress as AntProgress, Timeline, Statistic, Tooltip } from 'antd';
import { CheckCircleOutlined, ClockCircleOutlined } from '@ant-design/icons';
import { useParams, Link } from 'react-router-dom';
import { usePageTitle } from '../../hooks/usePageTitle';

interface LearnerData {
  user_id: string;
  username: string;
  display_name: string;
  email?: string;
  role: string;
  last_login_at?: string;
  created_at: string;
  enrolled_paths: Array<{
    path_id: string;
    path_slug: string;
    path_title: string;
    completed: number;
    total: number;
    percentage: number;
  }>;
  activity: Array<{
    step_title: string;
    path_title: string;
    event: string;
    timestamp: string;
  }>;
  performance: Array<{
    step_id: string;
    step_title: string;
    step_type: string;
    attempts: number;
    correct: number;
  }>;
  kpis: {
    enrolled_paths: number;
    completed_paths: number;
    completed_steps: number;
    enrolled_steps: number;
    stuck_steps: number;
    progress_rate: number | null;
    first_try_rate: number | null;
    avg_attempts: number | null;
    inactive: boolean;
    active_days_30d: number;
    current_streak: number;
    inactive_days: number;
    stuck_days: number;
  };
}

const LearnerDetail: React.FC = () => {
  usePageTitle('Learner Detail');
  const { learnerId } = useParams<{ learnerId: string }>();
  const [data, setData] = useState<LearnerData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch(`/api/analytics/learners/${learnerId}`, { credentials: 'include' })
      .then((r) => r.json())
      .then(setData)
      .finally(() => setLoading(false));
  }, [learnerId]);

  if (loading || !data) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  const formatDay = (iso: string) => {
    const d = new Date(iso);
    const today = new Date();
    const yesterday = new Date(today);
    yesterday.setDate(yesterday.getDate() - 1);
    if (d.toDateString() === today.toDateString()) return 'Today';
    if (d.toDateString() === yesterday.toDateString()) return 'Yesterday';
    return d.toLocaleDateString(undefined, { weekday: 'long', month: 'short', day: 'numeric' });
  };

  const kpis = data.kpis;

  // Group activity by day
  const groupedActivity: Record<string, typeof data.activity> = {};
  for (const a of data.activity) {
    const day = formatDay(a.timestamp);
    (groupedActivity[day] ??= []).push(a);
  }

  return (
    <div>
      <Breadcrumb items={[
        { title: <Link to="/analytics">Analytics</Link> },
        { title: <Link to="/analytics/learners">Learners</Link> },
        { title: data.display_name || data.username },
      ]} style={{ marginBottom: 16 }} />

      <Typography.Title level={2}>
        {data.display_name || data.username}
        {kpis.inactive && (
          <Tooltip title={`No activity for more than ${kpis.inactive_days} days`}>
            <Tag color="orange" style={{ marginLeft: 12, verticalAlign: 'middle' }}>Inactive</Tag>
          </Tooltip>
        )}
        {kpis.stuck_steps > 0 && (
          <Tooltip title={`${kpis.stuck_steps} step(s) started more than ${kpis.stuck_days} days ago and not completed`}>
            <Tag color="red" style={{ marginLeft: 12, verticalAlign: 'middle' }}>Stuck</Tag>
          </Tooltip>
        )}
      </Typography.Title>

      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={12} md={8} xl={4}>
          <Card><Statistic title="Paths completed" value={kpis.completed_paths} suffix={`/ ${kpis.enrolled_paths}`} /></Card>
        </Col>
        <Col xs={12} md={8} xl={4}>
          <Tooltip title={`${kpis.completed_steps} of ${kpis.enrolled_steps} steps in enrolled paths`}>
            <Card><Statistic title="Progress" value={kpis.progress_rate === null ? '—' : Math.round(kpis.progress_rate)} suffix={kpis.progress_rate === null ? undefined : '%'} /></Card>
          </Tooltip>
        </Col>
        <Col xs={12} md={8} xl={4}>
          <Tooltip title="Share of exercises passed at the first attempt">
            <Card><Statistic title="First-try success" value={kpis.first_try_rate === null ? '—' : Math.round(kpis.first_try_rate)} suffix={kpis.first_try_rate === null ? undefined : '%'} /></Card>
          </Tooltip>
        </Col>
        <Col xs={12} md={8} xl={4}>
          <Card><Statistic title="Avg attempts / exercise" value={kpis.avg_attempts === null ? '—' : kpis.avg_attempts.toFixed(1)} /></Card>
        </Col>
        <Col xs={12} md={8} xl={4}>
          <Card><Statistic title="Active days (30d)" value={kpis.active_days_30d} /></Card>
        </Col>
        <Col xs={12} md={8} xl={4}>
          <Tooltip title="Consecutive days with activity, up to today or yesterday">
            <Card><Statistic title="Current streak" value={kpis.current_streak} suffix={kpis.current_streak === 1 ? 'day' : 'days'} /></Card>
          </Tooltip>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={8}>
          <Card>
            <Typography.Text type="secondary">Role</Typography.Text>
            <div><Tag color={data.role === 'admin' ? 'red' : data.role === 'instructor' ? 'blue' : 'default'}>{data.role}</Tag></div>
            {data.email && <div style={{ marginTop: 8 }}><Typography.Text type="secondary">{data.email}</Typography.Text></div>}
            {data.last_login_at && (
              <div style={{ marginTop: 8 }}>
                <Typography.Text type="secondary">Last login: {new Date(data.last_login_at).toLocaleString()}</Typography.Text>
              </div>
            )}
            <div style={{ marginTop: 8 }}>
              <Typography.Text type="secondary">Joined: {new Date(data.created_at).toLocaleDateString()}</Typography.Text>
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={16}>
          <Card title="Enrolled Learning Paths">
            {data.enrolled_paths.length === 0 ? (
              <Typography.Text type="secondary">No enrollments yet</Typography.Text>
            ) : (
              data.enrolled_paths.map((ep) => (
                <div key={ep.path_id} style={{ marginBottom: 16 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <Link to={`/analytics/paths/${ep.path_slug}`}>{ep.path_title}</Link>
                    <Typography.Text type="secondary">{ep.completed}/{ep.total} steps</Typography.Text>
                  </div>
                  <AntProgress percent={Math.round(ep.percentage)} size="small" />
                </div>
              ))
            )}
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <Card title="Activity Timeline" style={{ maxHeight: 500, overflow: 'auto' }}>
            {Object.entries(groupedActivity).map(([day, items]) => (
              <div key={day}>
                <Typography.Text strong style={{ display: 'block', marginBottom: 8, marginTop: 8 }}>{day}</Typography.Text>
                <Timeline
                  items={items.map((a) => ({
                    color: a.event === 'completed' ? 'green' : 'blue',
                    dot: a.event === 'completed' ? <CheckCircleOutlined /> : <ClockCircleOutlined />,
                    children: (
                      <div>
                        <Typography.Text>{a.event === 'completed' ? 'Completed' : 'Started'}: {a.step_title}</Typography.Text>
                        <br />
                        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                          {a.path_title} · {new Date(a.timestamp).toLocaleTimeString()}
                        </Typography.Text>
                      </div>
                    ),
                  }))}
                />
              </div>
            ))}
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title="Exercise Performance">
            <Table
              dataSource={data.performance}
              rowKey="step_id"
              pagination={false}
              size="small"
              columns={[
                { title: 'Exercise', dataIndex: 'step_title' },
                { title: 'Type', dataIndex: 'step_type', width: 120, render: (v: string) => <Tag>{v}</Tag> },
                { title: 'Attempts', dataIndex: 'attempts', width: 80 },
                { title: 'Correct', dataIndex: 'correct', width: 80 },
                {
                  title: 'Rate',
                  width: 80,
                  render: (_: unknown, r) => r.attempts > 0 ? `${Math.round((r.correct / r.attempts) * 100)}%` : '—',
                },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default LearnerDetail;
