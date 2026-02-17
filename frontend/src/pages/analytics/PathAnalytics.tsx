import React, { useEffect, useState } from 'react';
import { Typography, Card, Table, Spin, Tag, Breadcrumb, Row, Col, Statistic, Progress as AntProgress, Tabs } from 'antd';
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

interface StepDrillDown {
  common_wrong_answers?: Array<{ answer: string; count: number }>;
}

const PathAnalyticsView: React.FC = () => {
  const { pathId } = useParams<{ pathId: string }>();
  const navigate = useNavigate();
  const [data, setData] = useState<PathAnalytics | null>(null);
  const [loading, setLoading] = useState(true);
  const [stepDetail, setStepDetail] = useState<Record<string, StepDrillDown>>({});

  useEffect(() => {
    fetch(`/api/analytics/paths/${pathId}`, { credentials: 'include' })
      .then((r) => r.json())
      .then(setData)
      .finally(() => setLoading(false));
  }, [pathId]);

  const loadStepDetail = async (stepId: string) => {
    if (stepDetail[stepId]) return;
    const resp = await fetch(`/api/analytics/paths/${pathId}/steps/${stepId}`, { credentials: 'include' });
    const d = await resp.json();
    setStepDetail((prev) => ({ ...prev, [stepId]: d }));
  };

  if (loading || !data) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  return (
    <div>
      <Breadcrumb items={[
        { title: <Link to="/analytics">Analytics</Link> },
        { title: data.title },
      ]} style={{ marginBottom: 16 }} />

      <Typography.Title level={2}>{data.title}</Typography.Title>

      <Tabs defaultActiveKey="overview" items={[
        {
          key: 'overview',
          label: 'Overview',
          children: (
            <>
              <Row gutter={[16, 16]}>
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
              <Card title="Step Completion Distribution" style={{ marginTop: 16 }}>
                {data.steps.map((s) => (
                  <div key={s.id} style={{ marginBottom: 8 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 2 }}>
                      <Typography.Text ellipsis style={{ maxWidth: '70%' }}>{s.title}</Typography.Text>
                      <Typography.Text type="secondary">{s.completion_rate.toFixed(0)}%</Typography.Text>
                    </div>
                    <AntProgress percent={Math.round(s.completion_rate)} size="small" showInfo={false}
                      strokeColor={s.completion_rate < 60 ? '#faad14' : '#52c41a'} />
                  </div>
                ))}
              </Card>
            </>
          ),
        },
        {
          key: 'steps',
          label: 'Step Details',
          children: (
            <Table
              dataSource={data.steps}
              rowKey="id"
              pagination={false}
              size="small"
              expandable={{
                expandedRowRender: (record) => {
                  const detail = stepDetail[record.id];
                  if (!detail) return <Spin size="small" />;
                  const wrongAnswers = detail.common_wrong_answers || [];
                  if (wrongAnswers.length === 0) return <Typography.Text type="secondary">No wrong answer data</Typography.Text>;
                  return (
                    <div>
                      <Typography.Text strong>Common Wrong Answers:</Typography.Text>
                      <Table
                        dataSource={wrongAnswers}
                        rowKey="answer"
                        size="small"
                        pagination={false}
                        columns={[
                          { title: 'Answer', dataIndex: 'answer' },
                          { title: 'Count', dataIndex: 'count', width: 80 },
                        ]}
                      />
                    </div>
                  );
                },
                onExpand: (expanded, record) => { if (expanded) loadStepDetail(record.id); },
              }}
              columns={[
                { title: 'Type', dataIndex: 'type', width: 120, render: (v: string) => <Tag>{v}</Tag> },
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
          ),
        },
        {
          key: 'learners',
          label: 'Learners',
          children: (
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
                { title: 'Completed', width: 120, render: (_: unknown, r) => `${r.completed}/${r.total}` },
              ]}
            />
          ),
        },
      ]} />
    </div>
  );
};

export default PathAnalyticsView;
