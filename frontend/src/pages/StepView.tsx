import React, { useEffect, useState, useCallback, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Layout, Menu, Spin, Button, Typography, Popconfirm, message, Modal } from 'antd';
import {
  ArrowLeftOutlined, ArrowRightOutlined, MenuFoldOutlined, MenuUnfoldOutlined,
  FileTextOutlined, QuestionCircleOutlined, DesktopOutlined, CodeOutlined,
  CheckCircleOutlined, ClockCircleOutlined, CloseOutlined,
} from '@ant-design/icons';
import { api } from '../api/client';
import type { StepDetail, LearningPathDetail, Progress } from '../api/client';
import { usePageTitle } from '../hooks/usePageTitle';
import MarkdownRenderer from '../components/MarkdownRenderer';
import Quiz from '../components/Quiz';
import TerminalExercise from '../components/TerminalExercise';
import CodeExercise from '../components/CodeExercise';

const { Content } = Layout;

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
  const [sidebarWidth, setSidebarWidth] = useState(280);
  const [exerciseCompleted, setExerciseCompleted] = useState(false);
  const sidebarRef = useRef<HTMLDivElement>(null);
  const resizingRef = useRef(false);
  const contentRef = useRef<HTMLDivElement>(null);
  const scrollTrackedRef = useRef(false);
  usePageTitle(step && path ? `${step.title} — ${path.title}` : 'Step');

  useEffect(() => {
    if (!pathId || !stepId) return;
    setLoading(true);
    setExerciseCompleted(false);
    scrollTrackedRef.current = false;
    Promise.all([
      api.getPath(pathId),
      api.getStep(pathId, stepId),
      api.getProgress(pathId),
    ])
      .then(([p, s, pr]) => {
        setPath(p);
        setStep(s);
        setProgress(pr);
        // Auto in_progress only for non-lesson steps (exercises/quizzes)
        if (s.type !== 'lesson') {
          api.updateProgress(stepId, 'in_progress').catch(() => {});
        }
      })
      .finally(() => setLoading(false));
  }, [pathId, stepId]);

  // Scroll tracking for lessons: mark in_progress at 75% scroll
  useEffect(() => {
    if (!step || step.type !== 'lesson' || !stepId) return;
    const status = getStepStatus(step.id);
    if (status === 'completed' || status === 'in_progress') return;

    const container = contentRef.current;
    if (!container) return;

    const handleScroll = () => {
      if (scrollTrackedRef.current) return;
      const { scrollTop, scrollHeight, clientHeight } = container;
      const scrollableHeight = scrollHeight - clientHeight;
      if (scrollableHeight <= 0) return; // Content fits without scroll
      const scrollPercent = scrollTop / scrollableHeight;
      if (scrollPercent >= 0.75) {
        scrollTrackedRef.current = true;
        api.updateProgress(stepId, 'in_progress').catch(() => {});
        setProgress((prev) => {
          const existing = prev.find((p) => p.step_id === stepId);
          if (existing) {
            return prev.map((p) => p.step_id === stepId ? { ...p, status: 'in_progress' as const } : p);
          }
          return [...prev, { id: '', user_id: '', step_id: stepId, status: 'in_progress' as const }];
        });
      }
    };

    // Also check if content is short enough that no scroll is needed — auto-mark
    const checkNoScroll = () => {
      if (scrollTrackedRef.current) return;
      const { scrollHeight, clientHeight } = container;
      if (scrollHeight <= clientHeight) {
        scrollTrackedRef.current = true;
        api.updateProgress(stepId, 'in_progress').catch(() => {});
        setProgress((prev) => {
          const existing = prev.find((p) => p.step_id === stepId);
          if (existing) {
            return prev.map((p) => p.step_id === stepId ? { ...p, status: 'in_progress' as const } : p);
          }
          return [...prev, { id: '', user_id: '', step_id: stepId, status: 'in_progress' as const }];
        });
      }
    };

    // Delay check for short content (DOM needs to render)
    const timer = setTimeout(checkNoScroll, 500);
    container.addEventListener('scroll', handleScroll, { passive: true });
    return () => {
      clearTimeout(timer);
      container.removeEventListener('scroll', handleScroll);
    };
  }, [step, stepId, progress]);

  const allSteps = path?.modules.flatMap((m) =>
    m.steps.map((s) => ({ ...s, moduleTitle: m.title, moduleId: m.id }))
  ) || [];
  const currentIdx = allSteps.findIndex((s) => s.slug === stepId || s.id === stepId);
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

    if (nextStep && path) {
      navigate(`/paths/${path.slug}/steps/${nextStep.slug}`);
    } else if (path) {
      // Last step — show congratulations
      Modal.success({
        title: '🎉 Congratulations!',
        content: `You've completed all steps in "${path.title}". Well done!`,
        okText: 'Back to Overview',
        onOk: () => navigate(`/paths/${path.slug}`),
      });
    }
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
        key: s.slug,
        icon: status === 'completed'
          ? <CheckCircleOutlined style={{ color: 'var(--color-success)' }} />
          : status === 'in_progress'
          ? <ClockCircleOutlined style={{ color: 'var(--color-warning)' }} />
          : stepIcon(s.type),
        label: <span style={(s.slug === stepId || s.id === stepId) ? { fontWeight: 'bold' } : undefined}>{s.title}</span>,
      };
    }),
  }));

  const exerciseData = step.exercise_data as Record<string, unknown> | undefined;
  const isCompleted = getStepStatus(step.id) === 'completed';

  return (
    <Layout style={{ height: 'calc(100vh - 64px)', margin: '-24px -48px', flexDirection: 'row', overflow: 'hidden' }}>
      <div style={{
        width: collapsed ? 48 : sidebarWidth, flexShrink: 0, display: 'flex',
        background: 'var(--color-bg-sidebar)', borderRight: '1px solid var(--color-border)', position: 'relative',
        height: '100%', overflow: 'hidden',
        transition: collapsed ? 'width 0.2s' : undefined,
      }}>
        <div style={{ flex: 1, overflow: 'auto' }}>
        <div ref={sidebarRef} style={{ height: '100%', overflow: 'auto' }}>
        <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--color-border)', display: 'flex', alignItems: 'center', gap: 8 }}>
          <Button
            type="text"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={() => setCollapsed(!collapsed)}
          />
          {!collapsed && (
            <>
              <Button type="link" onClick={() => navigate(`/paths/${path.slug}`)} style={{ flex: 1, textAlign: 'left', padding: 0 }}>
                ← {path.title}
              </Button>
              <Button type="text" icon={<CloseOutlined />} size="small" onClick={() => navigate(`/paths/${path.slug}`)} />
            </>
          )}
        </div>
        {!collapsed && (
          <Menu
            mode="inline"
            selectedKeys={[step.slug || stepId || '']}
            openKeys={path.modules.map((m) => m.id)}
            items={sidebarItems}
            onClick={({ key }) => navigate(`/paths/${path.slug}/steps/${key}`)}
          />
        )}
        </div>
        </div>
        {/* Resize handle */}
        {!collapsed && (
          <div
            onMouseDown={(e) => {
              e.preventDefault();
              resizingRef.current = true;
              const startX = e.clientX;
              const startW = sidebarWidth;
              const onMove = (ev: MouseEvent) => {
                if (!resizingRef.current) return;
                setSidebarWidth(Math.max(180, Math.min(500, startW + ev.clientX - startX)));
              };
              const onUp = () => {
                resizingRef.current = false;
                window.removeEventListener('mousemove', onMove);
                window.removeEventListener('mouseup', onUp);
              };
              window.addEventListener('mousemove', onMove);
              window.addEventListener('mouseup', onUp);
            }}
            style={{
              width: 4, cursor: 'col-resize', background: 'transparent',
              position: 'absolute', right: 0, top: 0, bottom: 0, zIndex: 10,
            }}
            onMouseEnter={(e) => (e.currentTarget.style.background = 'var(--color-resize-handle-hover)')}
            onMouseLeave={(e) => { if (!resizingRef.current) e.currentTarget.style.background = 'transparent'; }}
          />
        )}
      </div>
      <Content ref={contentRef} style={step.type === 'code-exercise' ? { padding: 0, overflow: 'hidden' } : { padding: 24, overflow: 'auto' }}>
        {step.type === 'code-exercise' && exerciseData ? (
          <CodeExercise
            mode={exerciseData.mode as string || 'A'}
            description={exerciseData.description as string || ''}
            target={exerciseData.target as any}
            patches={(exerciseData.patches as Record<string, unknown>[]).map((p) => p as any)}
            codebaseFiles={step.codebase_files || []}
            onSubmit={handleSubmitAttempt}
            onNext={nextStep ? () => navigate(`/paths/${pathId}/steps/${nextStep.id}`) : undefined}
            onBack={prevStep ? () => navigate(`/paths/${pathId}/steps/${prevStep.id}`) : undefined}
            onOverview={() => navigate(`/paths/${pathId}`)}
            nextLabel={nextStep?.title}
            prevLabel={prevStep?.title}
            onReset={handleReset}
          />
        ) : (
        <div style={{ maxWidth: 900, margin: '0 auto' }}>
          <Typography.Title level={3}>{step.title}</Typography.Title>

          {/* Completed banner for lessons */}
          {step.type === 'lesson' && isCompleted && (
            <div style={{ marginBottom: 16, padding: '8px 16px', background: '#f6ffed', border: '1px solid #b7eb8f', borderRadius: 6, display: 'flex', alignItems: 'center', gap: 8 }}>
              <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 16 }} />
              <Typography.Text style={{ color: '#52c41a', fontWeight: 500 }}>Completed</Typography.Text>
            </div>
          )}

          {/* Step content by type */}
          {step.type === 'lesson' && (
            <MarkdownRenderer content={step.content_md} />
          )}

          {step.type === 'quiz' && exerciseData && (
            <Quiz
              questions={(exerciseData.questions as Record<string, unknown>[]).map((q) => q as any)}
              onSubmit={handleSubmitAttempt}
              onComplete={() => setExerciseCompleted(true)}
            />
          )}

          {step.type === 'terminal-exercise' && exerciseData && (
            <TerminalExercise
              introduction={exerciseData.introduction as string || ''}
              steps={(exerciseData.steps as Record<string, unknown>[]).map((s) => s as any)}
              onSubmit={handleSubmitAttempt}
              onComplete={() => setExerciseCompleted(true)}
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
              <Button icon={<ArrowLeftOutlined />} onClick={() => navigate(`/paths/${path.slug}/steps/${prevStep.slug}`)}>
                {prevStep.title}
              </Button>
            ) : <div />}
            {step.type === 'lesson' ? (
              isCompleted ? (
                nextStep ? (
                  <Button type="primary" onClick={() => navigate(`/paths/${path.slug}/steps/${nextStep.slug}`)}>
                    Continue <ArrowRightOutlined />
                  </Button>
                ) : (
                  <Button type="primary" onClick={() => navigate(`/paths/${path.slug}`)}>
                    Back to Overview
                  </Button>
                )
              ) : (
                <Button
                  type="primary"
                  disabled={getStepStatus(step.id) === 'not_started'}
                  onClick={handleLessonComplete}
                >
                  {nextStep ? 'Complete lesson & Continue' : 'Complete lesson'} <ArrowRightOutlined />
                </Button>
              )
            ) : (
              nextStep ? (
                <Button
                  type="primary"
                  disabled={!isCompleted && !exerciseCompleted}
                  onClick={() => navigate(`/paths/${path.slug}/steps/${nextStep.slug}`)}
                >
                  {nextStep.title} <ArrowRightOutlined />
                </Button>
              ) : (
                <Button
                  type="primary"
                  disabled={!isCompleted && !exerciseCompleted}
                  onClick={() => navigate(`/paths/${path.slug}`)}
                >
                  Back to Overview
                </Button>
              )
            )}
          </div>
        </div>
        )}
      </Content>
    </Layout>
  );
};

export default StepView;
