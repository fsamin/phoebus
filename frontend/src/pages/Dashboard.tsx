import React, { useEffect, useState } from 'react';
import { Card, Row, Col, Typography, List, Tag, Spin, Empty } from 'antd';
import { BookOutlined, CheckCircleOutlined, PlayCircleOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { api } from '../api/client';
import type { LearningPathSummary, Progress } from '../api/client';
import { useAuth } from '../contexts/AuthContext';

const Dashboard: React.FC = () => {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [paths, setPaths] = useState<LearningPathSummary[]>([]);
  const [progress, setProgress] = useState<Progress[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([api.listPaths(), api.getProgress()])
      .then(([p, pr]) => { setPaths(p); setProgress(pr); })
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  const completedSteps = progress.filter((p) => p.status === 'completed').length;
  const inProgressSteps = progress.filter((p) => p.status === 'in_progress').length;

  return (
    <div>
      <Typography.Title level={2}>
        Welcome back, {user?.display_name || user?.username}!
      </Typography.Title>

      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={8}>
          <Card>
            <Card.Meta
              avatar={<BookOutlined style={{ fontSize: 32, color: '#1890ff' }} />}
              title={String(paths.length)}
              description="Learning Paths"
            />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card>
            <Card.Meta
              avatar={<CheckCircleOutlined style={{ fontSize: 32, color: '#52c41a' }} />}
              title={String(completedSteps)}
              description="Steps Completed"
            />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card>
            <Card.Meta
              avatar={<PlayCircleOutlined style={{ fontSize: 32, color: '#faad14' }} />}
              title={String(inProgressSteps)}
              description="Steps In Progress"
            />
          </Card>
        </Col>
      </Row>

      <Typography.Title level={4}>Learning Paths</Typography.Title>
      {paths.length === 0 ? (
        <Empty description="No learning paths available yet" />
      ) : (
        <List
          grid={{ gutter: 16, xs: 1, sm: 2, lg: 3 }}
          dataSource={paths}
          renderItem={(path) => (
            <List.Item>
              <Card
                hoverable
                onClick={() => navigate(`/paths/${path.id}`)}
                actions={[
                  <span key="modules">{path.module_count} modules</span>,
                  <span key="steps">{path.step_count} steps</span>,
                ]}
              >
                <Card.Meta
                  title={
                    <span>
                      {path.icon && <span style={{ marginRight: 8 }}>{path.icon}</span>}
                      {path.title}
                    </span>
                  }
                  description={path.description}
                />
                <div style={{ marginTop: 12 }}>
                  {path.tags?.map((tag) => <Tag key={tag}>{tag}</Tag>)}
                </div>
                {path.estimated_duration && (
                  <Typography.Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 8 }}>
                    ⏱ {path.estimated_duration}
                  </Typography.Text>
                )}
              </Card>
            </List.Item>
          )}
        />
      )}
    </div>
  );
};

export default Dashboard;
