import React, { useEffect, useState } from 'react';
import {
  Typography, Spin, Card, Collapse, List, Tag, Progress as AntProgress, Button,
} from 'antd';
import {
  BookOutlined, CheckCircleOutlined, PlayCircleOutlined,
  FileTextOutlined, QuestionCircleOutlined, CodeOutlined, DesktopOutlined,
} from '@ant-design/icons';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../api/client';
import type { LearningPathDetail, Progress, StepSummary } from '../api/client';

const stepIcon = (type: string) => {
  switch (type) {
    case 'lesson': return <FileTextOutlined />;
    case 'quiz': return <QuestionCircleOutlined />;
    case 'terminal-exercise': return <DesktopOutlined />;
    case 'code-exercise': return <CodeOutlined />;
    default: return <FileTextOutlined />;
  }
};

const stepStatus = (stepId: string, progress: Progress[]) => {
  const p = progress.find((pr) => pr.step_id === stepId);
  if (!p) return 'not_started';
  return p.status;
};

const PathOverview: React.FC = () => {
  const { pathId } = useParams<{ pathId: string }>();
  const navigate = useNavigate();
  const [path, setPath] = useState<LearningPathDetail | null>(null);
  const [progress, setProgress] = useState<Progress[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!pathId) return;
    Promise.all([api.getPath(pathId), api.getProgress(pathId)])
      .then(([p, pr]) => { setPath(p); setProgress(pr); })
      .finally(() => setLoading(false));
  }, [pathId]);

  if (loading || !path) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  const allSteps = path.modules.flatMap((m) => m.steps);
  const completedSteps = allSteps.filter((s) => stepStatus(s.id, progress) === 'completed').length;
  const pct = allSteps.length > 0 ? Math.round((completedSteps / allSteps.length) * 100) : 0;

  // Find first incomplete step for "Continue Learning" button
  const nextStep = allSteps.find((s) => stepStatus(s.id, progress) !== 'completed');

  return (
    <div>
      <Typography.Title level={2}>
        {path.icon && <span style={{ marginRight: 8 }}>{path.icon}</span>}
        {path.title}
      </Typography.Title>
      <Typography.Paragraph type="secondary">{path.description}</Typography.Paragraph>

      <Card style={{ marginBottom: 24 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 24 }}>
          <div style={{ flex: 1 }}>
            <AntProgress percent={pct} />
            <Typography.Text type="secondary">
              {completedSteps}/{allSteps.length} steps completed
            </Typography.Text>
          </div>
          {nextStep && (
            <Button type="primary" onClick={() => navigate(`/paths/${pathId}/steps/${nextStep.id}`)}>
              {completedSteps > 0 ? 'Continue Learning' : 'Start Learning'}
            </Button>
          )}
        </div>
        <div style={{ marginTop: 12 }}>
          {path.tags?.map((tag) => <Tag key={tag}>{tag}</Tag>)}
          {path.estimated_duration && (
            <Tag icon={<BookOutlined />}>{path.estimated_duration}</Tag>
          )}
        </div>
      </Card>

      <Collapse
        defaultActiveKey={path.modules.map((m) => m.id)}
        items={path.modules.map((mod) => {
          const modCompleted = mod.steps.filter((s) => stepStatus(s.id, progress) === 'completed').length;
          return {
            key: mod.id,
            label: (
              <div style={{ display: 'flex', justifyContent: 'space-between', width: '100%' }}>
                <span>
                  <strong>{mod.title}</strong>
                  <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
                    {modCompleted}/{mod.steps.length}
                  </Typography.Text>
                </span>
                <span>
                  {mod.competencies?.map((c) => <Tag key={c} color="blue">{c}</Tag>)}
                </span>
              </div>
            ),
            children: (
              <List
                dataSource={mod.steps}
                renderItem={(step: StepSummary) => {
                  const status = stepStatus(step.id, progress);
                  return (
                    <List.Item
                      style={{ cursor: 'pointer' }}
                      onClick={() => navigate(`/paths/${pathId}/steps/${step.id}`)}
                      extra={
                        status === 'completed' ? (
                          <CheckCircleOutlined style={{ color: '#52c41a' }} />
                        ) : status === 'in_progress' ? (
                          <PlayCircleOutlined style={{ color: '#faad14' }} />
                        ) : null
                      }
                    >
                      <List.Item.Meta
                        avatar={stepIcon(step.type)}
                        title={step.title}
                        description={
                          <span>
                            <Tag>{step.type}</Tag>
                            {step.estimated_duration && (
                              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                                ⏱ {step.estimated_duration}
                              </Typography.Text>
                            )}
                          </span>
                        }
                      />
                    </List.Item>
                  );
                }}
              />
            ),
          };
        })}
      />
    </div>
  );
};

export default PathOverview;
