import React, { useEffect, useState } from 'react';
import {
  Typography, Spin, Card, Collapse, List, Tag, Progress as AntProgress, Button, Breadcrumb, Modal,
} from 'antd';
import {
  CheckCircleOutlined, ClockCircleOutlined,
  FileTextOutlined, QuestionCircleOutlined, CodeOutlined, DesktopOutlined,
  WarningOutlined, ExclamationCircleOutlined,
} from '@ant-design/icons';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { api } from '../api/client';
import { usePageTitle } from '../hooks/usePageTitle';
import type { LearningPathDetail, Progress, StepSummary, LearningPathSummary } from '../api/client';

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
  const [allPaths, setAllPaths] = useState<LearningPathSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [prereqModalOpen, setPrereqModalOpen] = useState(false);
  const [pendingStepId, setPendingStepId] = useState<string | null>(null);
  usePageTitle(path ? path.title : 'Learning Path');

  useEffect(() => {
    if (!pathId) return;
    Promise.all([api.getPath(pathId), api.getProgress(pathId), api.listPaths()])
      .then(([p, pr, ap]) => { setPath(p); setProgress(pr); setAllPaths(ap); })
      .finally(() => setLoading(false));
  }, [pathId]);

  if (loading || !path) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  // Find current path in allPaths to get prerequisites_met
  const currentPathSummary = allPaths.find((p) => p.slug === pathId || p.id === pathId);
  const prerequisitesMet = currentPathSummary?.prerequisites_met ?? true;
  const dismissedKey = `prereq-dismissed-${path.slug}`;
  const isDismissed = () => sessionStorage.getItem(dismissedKey) === 'true';

  // Compute unmet prerequisites with their provider paths
  const unmetPrereqs = (path.prerequisites || []).map((prereq) => {
    const provider = allPaths.find(
      (p) => p.competencies_provided?.includes(prereq)
    );
    const providerCompleted = provider
      ? provider.progress_total && provider.progress_completed === provider.progress_total
      : false;
    return { competency: prereq, provider, met: !!providerCompleted };
  });

  const handleStartLearning = (stepSlug: string) => {
    if (!prerequisitesMet && !isDismissed()) {
      setPendingStepId(stepSlug);
      setPrereqModalOpen(true);
    } else {
      navigate(`/paths/${path.slug}/steps/${stepSlug}`);
    }
  };

  const handleContinueAnyway = () => {
    sessionStorage.setItem(dismissedKey, 'true');
    setPrereqModalOpen(false);
    if (pendingStepId) {
      navigate(`/paths/${path.slug}/steps/${pendingStepId}`);
    }
  };

  const handleBrowsePrereqs = () => {
    setPrereqModalOpen(false);
    const unmetCompetencies = unmetPrereqs.filter((p) => !p.met).map((p) => p.competency);
    navigate(`/catalog?competencies=${unmetCompetencies.join(',')}`);
  };

  if (loading || !path) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  const allSteps = path.modules.flatMap((m) => m.steps);
  const completedSteps = allSteps.filter((s) => stepStatus(s.id, progress) === 'completed').length;
  const pct = allSteps.length > 0 ? Math.round((completedSteps / allSteps.length) * 100) : 0;

  // Find first incomplete step for "Continue Learning" button
  const nextStep = allSteps.find((s) => stepStatus(s.id, progress) !== 'completed');
  const isPathCompleted = allSteps.length > 0 && !nextStep;

  return (
    <div>
      <Breadcrumb items={[
        { title: <Link to="/catalog">Catalog</Link> },
        { title: path.title },
      ]} style={{ marginBottom: 16 }} />
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
              {' · '}{path.modules.length} modules
              {path.estimated_duration ? ` · ⏱ ${path.estimated_duration}` : ''}
            </Typography.Text>
          </div>
          {!isPathCompleted && nextStep && (
            <Button type="primary" onClick={() => handleStartLearning(nextStep.slug)}>
              {completedSteps > 0 ? 'Continue Learning' : 'Start Learning'}
            </Button>
          )}
          {isPathCompleted && <Tag color="green" icon={<CheckCircleOutlined />}>Path Completed</Tag>}
        </div>
        <div style={{ marginTop: 12 }}>
          {path.tags?.map((tag) => <Tag key={tag}>{tag}</Tag>)}
        </div>
        {path.prerequisites && path.prerequisites.length > 0 && (
          <div style={{ marginTop: 12 }}>
            <Typography.Text type="secondary">Prerequisites: </Typography.Text>
            {unmetPrereqs.map((p) => (
              <Tag
                key={p.competency}
                icon={p.met ? <CheckCircleOutlined /> : <WarningOutlined />}
                color={p.met ? 'success' : 'warning'}
              >
                {p.competency}
                {p.provider && (
                  <Typography.Text type="secondary" style={{ fontSize: 11, marginLeft: 4 }}>
                    ({p.provider.title})
                  </Typography.Text>
                )}
              </Tag>
            ))}
          </div>
        )}
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
                      onClick={() => handleStartLearning(step.slug)}
                      extra={
                        <>
                          {status === 'completed' ? (
                            <CheckCircleOutlined style={{ color: 'var(--color-success)' }} />
                          ) : status === 'in_progress' ? (
                            <span><ClockCircleOutlined style={{ color: 'var(--color-warning)' }} /> <Typography.Text type="secondary" style={{ fontSize: 12 }}>← in progress</Typography.Text></span>
                          ) : null}
                        </>
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

      <Modal
        title={
          <span>
            <ExclamationCircleOutlined style={{ color: '#faad14', marginRight: 8 }} />
            Prerequisites Not Met
          </span>
        }
        open={prereqModalOpen}
        onCancel={() => setPrereqModalOpen(false)}
        footer={[
          <Button key="browse" onClick={handleBrowsePrereqs}>
            Browse Prerequisite Paths
          </Button>,
          <Button key="continue" type="primary" onClick={handleContinueAnyway}>
            Continue Anyway
          </Button>,
        ]}
      >
        <Typography.Paragraph>
          This learning path requires knowledge of the following competencies:
        </Typography.Paragraph>
        <List
          size="small"
          dataSource={unmetPrereqs}
          renderItem={(item) => (
            <List.Item>
              <span>
                {item.met ? (
                  <CheckCircleOutlined style={{ color: 'var(--color-success)', marginRight: 8 }} />
                ) : (
                  <WarningOutlined style={{ color: '#faad14', marginRight: 8 }} />
                )}
                <strong>{item.competency}</strong>
                {item.provider && (
                  <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
                    (provided by "{item.provider.title}")
                  </Typography.Text>
                )}
                {item.met ? (
                  <Tag color="success" style={{ marginLeft: 8 }}>Completed</Tag>
                ) : (
                  <Tag color="warning" style={{ marginLeft: 8 }}>Not completed</Tag>
                )}
              </span>
            </List.Item>
          )}
        />
        <Typography.Paragraph type="secondary" style={{ marginTop: 16 }}>
          You may continue, but the content assumes familiarity with these topics.
        </Typography.Paragraph>
      </Modal>
    </div>
  );
};

export default PathOverview;
