import React, { useEffect, useState } from 'react';
import { Card, Row, Col, Typography, List, Tag, Spin, Empty, Progress as AntProgress, Timeline, Statistic, Button, message, Space, Tooltip } from 'antd';
import {
  CheckCircleOutlined, PlayCircleOutlined, TrophyOutlined,
  ExperimentOutlined, ArrowRightOutlined, SyncOutlined, UnorderedListOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { api } from '../api/client';
import { usePageTitle } from '../hooks/usePageTitle';
import OnboardingTour from '../components/OnboardingTour';
import { dashboardSteps } from '../tours/steps';

interface DashboardData {
  continue_learning: { step_id: string; step_title: string; path_id: string; path_title: string } | null;
  enrolled_paths: Array<{ path_id: string; path_title: string; path_icon: string; total: number; completed: number }>;
  competencies: Array<{ name: string; acquired: boolean; path_title: string }>;
  stats: { steps_completed: number; total_exercises: number; steps_in_progress: number };
  recent_activity: Array<{ step_title: string; path_title: string; path_id: string; step_id: string; event: string; timestamp: string }>;
  instructor_repos: Array<{ id: string; clone_url: string; branch: string; sync_status: string; sync_error?: string; last_synced_at?: string; path_titles: string[] }>;
}

const Dashboard: React.FC = () => {
  usePageTitle('Dashboard');
  const { user } = useAuth();
  const navigate = useNavigate();
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchDashboard = () =>
    fetch('/api/me/dashboard', { credentials: 'include' })
      .then((r) => r.json())
      .then(setData);

  useEffect(() => {
    fetchDashboard().finally(() => setLoading(false));
  }, []);

  // Poll while any instructor repo is syncing
  useEffect(() => {
    const hasSyncing = data?.instructor_repos?.some((r) => r.sync_status === 'syncing');
    if (!hasSyncing) return;
    const interval = setInterval(fetchDashboard, 3000);
    return () => clearInterval(interval);
  }, [data]);

  if (loading || !data) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  return (
    <div>
      <OnboardingTour tour="dashboard" steps={dashboardSteps} />
      <Typography.Title level={2} data-tour="dashboard-welcome">
        Welcome back, {user?.display_name || user?.username}!
      </Typography.Title>

      {/* Continue Learning */}
      {data.continue_learning && (
        <Card style={{ marginBottom: 24, borderLeft: '4px solid var(--color-primary)' }} data-tour="dashboard-continue">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
              <Typography.Text type="secondary">Continue Learning</Typography.Text>
              <div>
                <Typography.Text strong style={{ fontSize: 16 }}>{data.continue_learning.step_title}</Typography.Text>
                <Typography.Text type="secondary" style={{ marginLeft: 8 }}>in {data.continue_learning.path_title}</Typography.Text>
              </div>
            </div>
            <Button
              type="primary"
              icon={<ArrowRightOutlined />}
              onClick={() => navigate(`/paths/${data.continue_learning!.path_id}/steps/${data.continue_learning!.step_id}`)}
            >
              Resume
            </Button>
          </div>
        </Card>
      )}

      {/* Instructor Repos */}
      {data.instructor_repos?.length > 0 && (
        <Card title="My Repositories" style={{ marginBottom: 24 }}>
          <List
            dataSource={data.instructor_repos}
            renderItem={(repo) => (
              <List.Item
                actions={[
                  <Tooltip title="Sync now" key="sync">
                    <Button
                      size="small"
                      icon={<SyncOutlined />}
                      loading={repo.sync_status === 'syncing'}
                      onClick={async () => {
                        try {
                          await api.instructorSyncRepo(repo.id);
                          message.success('Sync triggered');
                          fetchDashboard();
                        } catch (e) {
                          message.error((e as Error).message);
                        }
                      }}
                    />
                  </Tooltip>,
                  <Tooltip title="Sync logs" key="logs">
                    <Button
                      size="small"
                      icon={<UnorderedListOutlined />}
                      onClick={() => navigate(`/instructor/repositories/${repo.id}/sync-logs`)}
                    />
                  </Tooltip>,
                ]}
              >
                <List.Item.Meta
                  title={
                    <Space>
                      <span>{repo.path_titles.length > 0 ? repo.path_titles.join(', ') : repo.clone_url}</span>
                      <Typography.Text type="secondary" style={{ fontWeight: 'normal' }}>{repo.branch}</Typography.Text>
                    </Space>
                  }
                  description={
                    <Space direction="vertical" size={2}>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }} copyable={{ text: repo.clone_url }}>
                        {repo.clone_url}
                      </Typography.Text>
                      <Space size="middle">
                        <Tag
                          color={repo.sync_status === 'synced' ? 'green' : repo.sync_status === 'syncing' ? 'blue' : repo.sync_status === 'error' ? 'red' : 'default'}
                          icon={repo.sync_status === 'syncing' ? <SyncOutlined spin /> : undefined}
                        >
                          {repo.sync_status}
                        </Tag>
                        {repo.last_synced_at && (
                          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                            Last synced: {new Date(repo.last_synced_at).toLocaleString()}
                          </Typography.Text>
                        )}
                      </Space>
                    </Space>
                  }
                />
              </List.Item>
            )}
          />
        </Card>
      )}

      {/* Stats */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }} data-tour="dashboard-stats">
        <Col xs={24} sm={8}>
          <Card><Statistic title="Steps Completed" value={data.stats.steps_completed} prefix={<CheckCircleOutlined />} /></Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card><Statistic title="Exercises Attempted" value={data.stats.total_exercises} prefix={<ExperimentOutlined />} /></Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card><Statistic title="Competencies" value={`${data.competencies.filter(c => c.acquired).length}/${data.competencies.length}`} prefix={<TrophyOutlined />} /></Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        {/* Enrolled Paths */}
        <Col xs={24} lg={12}>
          <Card title="My Learning Paths" style={{ marginBottom: 24 }} data-tour="dashboard-paths">
            {data.enrolled_paths.length === 0 ? (
              <Empty description="No enrolled paths yet" image={Empty.PRESENTED_IMAGE_SIMPLE}>
                <Button type="primary" onClick={() => navigate('/catalog')}>Browse Catalog</Button>
              </Empty>
            ) : (
              <List
                dataSource={data.enrolled_paths}
                renderItem={(ep) => (
                  <List.Item style={{ cursor: 'pointer' }} onClick={() => navigate(`/paths/${ep.path_id}`)}>
                    <div style={{ width: '100%' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Typography.Text strong>
                          {ep.path_icon && <span style={{ marginRight: 8 }}>{ep.path_icon}</span>}
                          {ep.path_title}
                        </Typography.Text>
                        <Typography.Text type="secondary">{ep.completed}/{ep.total}</Typography.Text>
                      </div>
                      <AntProgress percent={ep.total > 0 ? Math.round((ep.completed / ep.total) * 100) : 0} size="small" />
                    </div>
                  </List.Item>
                )}
              />
            )}
          </Card>

          {/* Competencies */}
          {data.competencies.length > 0 && (
            <Card title="Competencies" style={{ marginBottom: 24 }} data-tour="dashboard-competencies">
              {data.competencies.map((c, i) => (
                <Tag key={i} color={c.acquired ? 'green' : 'default'} style={{ marginBottom: 4 }}>
                  {c.acquired ? '✅' : '⬜'} {c.name}
                </Tag>
              ))}
            </Card>
          )}
        </Col>

        {/* Recent Activity */}
        <Col xs={24} lg={12}>
          <Card title="Recent Activity" data-tour="dashboard-activity">
            {data.recent_activity.length === 0 ? (
              <Empty description="No activity yet" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            ) : (
              <Timeline
                items={data.recent_activity.map((a) => ({
                  color: a.event === 'completed' ? 'green' : 'blue',
                  dot: a.event === 'completed' ? <CheckCircleOutlined /> : <PlayCircleOutlined />,
                  children: (
                    <div
                      style={{ cursor: 'pointer' }}
                      onClick={() => navigate(`/paths/${a.path_id}/steps/${a.step_id}`)}
                    >
                      <Typography.Text>{a.event === 'completed' ? 'Completed' : 'Started'}: {a.step_title}</Typography.Text>
                      <br />
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        {a.path_title} · {new Date(a.timestamp).toLocaleString()}
                      </Typography.Text>
                    </div>
                  ),
                }))}
              />
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
