import React, { useEffect, useState } from 'react';
import { Typography, Card, Table, Spin, Tag, Breadcrumb, Row, Col, Progress as AntProgress, Timeline } from 'antd';
import { CheckCircleOutlined, PlayCircleOutlined } from '@ant-design/icons';
import { useParams, Link } from 'react-router-dom';

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
}

const LearnerDetail: React.FC = () => {
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

  return (
    <div>
      <Breadcrumb items={[
        { title: <Link to="/analytics">Analytics</Link> },
        { title: data.display_name || data.username },
      ]} style={{ marginBottom: 16 }} />

      <Typography.Title level={2}>{data.display_name || data.username}</Typography.Title>

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
                    <Link to={`/analytics/paths/${ep.path_id}`}>{ep.path_title}</Link>
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
            <Timeline
              items={data.activity.map((a) => ({
                color: a.event === 'completed' ? 'green' : 'blue',
                dot: a.event === 'completed' ? <CheckCircleOutlined /> : <PlayCircleOutlined />,
                children: (
                  <div>
                    <Typography.Text>{a.event === 'completed' ? 'Completed' : 'Started'}: {a.step_title}</Typography.Text>
                    <br />
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      {a.path_title} · {new Date(a.timestamp).toLocaleString()}
                    </Typography.Text>
                  </div>
                ),
              }))}
            />
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
