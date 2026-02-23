import React, { useEffect, useState, useCallback, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Layout, Menu, Spin, Button, Typography, Popconfirm, message } from 'antd';
import {
  ArrowLeftOutlined, ArrowRightOutlined, MenuFoldOutlined, MenuUnfoldOutlined,
  FileTextOutlined, QuestionCircleOutlined, DesktopOutlined, CodeOutlined,
  CheckCircleOutlined, PlayCircleOutlined, CloseOutlined,
} from '@ant-design/icons';
import { api } from '../api/client';
import type { StepDetail, LearningPathDetail, Progress } from '../api/client';
import MarkdownRenderer from '../components/MarkdownRenderer';
import Quiz from '../components/Quiz';
import TerminalExercise from '../components/TerminalExercise';
import CodeExercise from '../components/CodeExercise';

const { Sider, Content } = Layout;

const stepIcon = (type: string) => {
  switch (type) {
    case 'lesson': return <FileTextOutlined />;
    case 'quiz': return <QuestionCircleOutlined />;
    case 'terminal-exercise': return <DesktopOutlined />;
    case 'code-exercise': return <CodeOutlined />;
    default: return <FileTextOutlined />;
  }
};

const StepView: React.FC = () => {
  const { pathId, stepId } = useParams<{ pathId: string; stepId: string }>();
  const navigate = useNavigate();
  const [path, setPath] = useState<LearningPathDetail | null>(null);
  const [step, setStep] = useState<StepDetail | null>(null);
  const [progress, setProgress] = useState<Progress[]>([]);
  const [loading, setLoading] = useState(true);
  const [collapsed, setCollapsed] = useState(false);
  const sidebarRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!pathId || !stepId) return;
    setLoading(true);
    Promise.all([
      api.getPath(pathId),
      api.getStep(pathId, stepId),
      api.getProgress(pathId),
    ])
      .then(([p, s, pr]) => {
        setPath(p);
        setStep(s);
        setProgress(pr);
        // Mark as in_progress
        api.updateProgress(stepId, 'in_progress').catch(() => {});
      })
      .finally(() => setLoading(false));
  }, [pathId, stepId]);

  const allSteps = path?.modules.flatMap((m) =>
    m.steps.map((s) => ({ ...s, moduleTitle: m.title, moduleId: m.id }))
  ) || [];
  const currentIdx = allSteps.findIndex((s) => s.id === stepId);
  const prevStep = currentIdx > 0 ? allSteps[currentIdx - 1] : null;
  const nextStep = currentIdx < allSteps.length - 1 ? allSteps[currentIdx + 1] : null;

  const getStepStatus = (sid: string) => {
    const p = progress.find((pr) => pr.step_id === sid);
    return p?.status || 'not_started';
  };

  const handleLessonComplete = async () => {
    if (!stepId) return;
    await api.updateProgress(stepId, 'completed');
    setProgress((prev) => {
      const existing = prev.find((p) => p.step_id === stepId);
      if (existing) {
        return prev.map((p) => p.step_id === stepId ? { ...p, status: 'completed' as const } : p);
      }
      return [...prev, { id: '', user_id: '', step_id: stepId, status: 'completed' as const }];
    });
    message.success('Step completed!');
  };

  const handleSubmitAttempt = useCallback(async (body: Record<string, unknown>) => {
    if (!stepId) throw new Error('No step ID');
    const result = await api.submitAttempt(stepId, body);
    // Refresh progress after attempt
    if (pathId) {
      api.getProgress(pathId).then(setProgress);
    }
    return result;
  }, [stepId, pathId]);

  const handleReset = async () => {
    if (!stepId) return;
    await api.resetExercise(stepId);
    message.info('Exercise reset');
    // Force reload the step
    window.location.reload();
  };

  if (loading || !path || !step) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  // Auto-scroll sidebar to current step
  setTimeout(() => {
    const el = sidebarRef.current?.querySelector('.ant-menu-item-selected');
    el?.scrollIntoView({ block: 'center', behavior: 'smooth' });
  }, 100);

  // Build sidebar menu items grouped by module
  const sidebarItems = path.modules.map((m) => ({
    key: m.id,
    label: m.title,
    children: m.steps.map((s) => {
      const status = getStepStatus(s.id);
      return {
        key: s.id,
        icon: status === 'completed'
          ? <CheckCircleOutlined style={{ color: '#52c41a' }} />
          : status === 'in_progress'
          ? <PlayCircleOutlined style={{ color: '#faad14' }} />
          : stepIcon(s.type),
        label: <span style={s.id === stepId ? { fontWeight: 'bold' } : undefined}>{s.title}</span>,
      };
    }),
  }));

  const exerciseData = step.exercise_data as Record<string, unknown> | undefined;
  const isCompleted = getStepStatus(step.id) === 'completed';

  return (
    <Layout style={{ minHeight: 'calc(100vh - 64px)', margin: '-24px -48px' }}>
      <Sider
        width={280}
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        trigger={null}
        theme="light"
        style={{ borderRight: '1px solid #f0f0f0', overflow: 'auto' }}
      >
        <div ref={sidebarRef} style={{ height: '100%', overflow: 'auto' }}>
        <div style={{ padding: '12px 16px', borderBottom: '1px solid #f0f0f0', display: 'flex', alignItems: 'center', gap: 8 }}>
          <Button
            type="text"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={() => setCollapsed(!collapsed)}
          />
          {!collapsed && (
            <>
              <Button type="link" onClick={() => navigate(`/paths/${pathId}`)} style={{ flex: 1, textAlign: 'left', padding: 0 }}>
                ← {path.title}
              </Button>
              <Button type="text" icon={<CloseOutlined />} size="small" onClick={() => navigate(`/paths/${pathId}`)} />
            </>
          )}
        </div>
        {!collapsed && (
          <Menu
            mode="inline"
            selectedKeys={[stepId || '']}
            openKeys={path.modules.map((m) => m.id)}
            items={sidebarItems}
            onClick={({ key }) => navigate(`/paths/${pathId}/steps/${key}`)}
          />
        )}
        </div>
      </Sider>
      <Content style={{ padding: 24, overflow: 'auto' }}>
        <div style={{ maxWidth: 900, margin: '0 auto' }}>
          <Typography.Title level={3}>{step.title}</Typography.Title>

          {/* Step content by type */}
          {step.type === 'lesson' && (
            <>
              <MarkdownRenderer content={step.content_md} />
              <div style={{ marginTop: 24, textAlign: 'center' }}>
                {isCompleted ? (
                  <Button type="primary" size="large" disabled icon={<CheckCircleOutlined />}>
                    ✅ Completed
                  </Button>
                ) : (
                  <Button type="primary" size="large" onClick={handleLessonComplete}>
                    Mark as Completed
                  </Button>
                )}
              </div>
            </>
          )}

          {step.type === 'quiz' && exerciseData && (
            <Quiz
              questions={(exerciseData.questions as Record<string, unknown>[]).map((q) => q as any)}
              onSubmit={handleSubmitAttempt}
            />
          )}

          {step.type === 'terminal-exercise' && exerciseData && (
            <TerminalExercise
              introduction={exerciseData.introduction as string || ''}
              steps={(exerciseData.steps as Record<string, unknown>[]).map((s) => s as any)}
              onSubmit={handleSubmitAttempt}
            />
          )}

          {step.type === 'code-exercise' && exerciseData && (
            <CodeExercise
              mode={exerciseData.mode as string || 'A'}
              description={exerciseData.description as string || ''}
              target={exerciseData.target as any}
              patches={(exerciseData.patches as Record<string, unknown>[]).map((p) => p as any)}
              codebaseFiles={step.codebase_files || []}
              onSubmit={handleSubmitAttempt}
            />
          )}

          {/* Reset button for exercises */}
          {step.type !== 'lesson' && (
            <div style={{ marginTop: 24, textAlign: 'center' }}>
              <Popconfirm title="Reset this exercise? Your previous attempts will be preserved." onConfirm={handleReset}>
                <Button danger>Reset Exercise</Button>
              </Popconfirm>
            </div>
          )}

          {/* Navigation footer */}
          <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 32, padding: '16px 0', borderTop: '1px solid #f0f0f0' }}>
            {prevStep ? (
              <Button icon={<ArrowLeftOutlined />} onClick={() => navigate(`/paths/${pathId}/steps/${prevStep.id}`)}>
                {prevStep.title}
              </Button>
            ) : <div />}
            {nextStep ? (
              <Button type="primary" onClick={() => navigate(`/paths/${pathId}/steps/${nextStep.id}`)}>
                {nextStep.title} <ArrowRightOutlined />
              </Button>
            ) : (
              <Button type="primary" onClick={() => navigate(`/paths/${pathId}`)}>
                Back to Overview
              </Button>
            )}
          </div>
        </div>
      </Content>
    </Layout>
  );
};

export default StepView;
