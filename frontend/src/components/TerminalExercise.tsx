import React, { useState } from 'react';
import { Card, Radio, Button, Alert, Typography, Tag, Space, Divider } from 'antd';
import MarkdownRenderer from './MarkdownRenderer';

interface Proposal {
  command: string;
  correct: boolean;
  explanation: string;
}

interface TerminalStep {
  context: string;
  prompt: string;
  output: string;
  proposals: Proposal[];
}

interface TerminalExerciseProps {
  introduction: string;
  steps: TerminalStep[];
  onSubmit: (body: Record<string, unknown>) => Promise<Record<string, unknown>>;
}

const TerminalExercise: React.FC<TerminalExerciseProps> = ({ introduction, steps, onSubmit }) => {
  const [currentStep, setCurrentStep] = useState(0);
  const [selected, setSelected] = useState('');
  const [feedback, setFeedback] = useState<Record<string, unknown> | null>(null);
  const [history, setHistory] = useState<Array<{ command: string; output: string }>>([]);
  const [submitting, setSubmitting] = useState(false);
  const [completed, setCompleted] = useState(false);
  const [disabledCommands, setDisabledCommands] = useState<Set<string>>(new Set());

  if (completed) {
    return (
      <Card title="Exercise Complete ✅">
        <div style={{ background: 'var(--color-bg-terminal)', padding: 16, borderRadius: 8, fontFamily: 'monospace', color: 'var(--color-text-terminal)' }}>
          {history.map((h, i) => (
            <div key={i} style={{ marginBottom: 8 }}>
              <div style={{ color: 'var(--color-text-terminal-cmd)' }}>$ {h.command}</div>
              {h.output && <div style={{ whiteSpace: 'pre-wrap', color: 'var(--color-text-terminal-output)' }}>{h.output}</div>}
            </div>
          ))}
        </div>
      </Card>
    );
  }

  const step = steps[currentStep];

  const handleSubmit = async () => {
    setSubmitting(true);
    try {
      const result = await onSubmit({ step_number: currentStep + 1, selected_command: selected });
      setFeedback(result);
      if (result.is_correct) {
        setHistory((prev) => [...prev, { command: selected, output: (result.output as string) || '' }]);
        setTimeout(() => {
          if (currentStep + 1 >= steps.length) {
            setCompleted(true);
          } else {
            setCurrentStep(currentStep + 1);
            setSelected('');
            setFeedback(null);
            setDisabledCommands(new Set());
          }
        }, 1500);
      } else {
        setDisabledCommands((prev) => new Set([...prev, selected]));
        setSelected('');
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Card
      title={
        <span>
          Terminal Exercise
          <Tag style={{ marginLeft: 12 }}>Step {currentStep + 1}/{steps.length}</Tag>
        </span>
      }
    >
      {introduction && currentStep === 0 && (
        <Typography.Paragraph type="secondary">
          <MarkdownRenderer content={introduction} />
        </Typography.Paragraph>
      )}

      {/* Terminal history */}
      <div style={{ background: 'var(--color-bg-terminal)', padding: 16, borderRadius: 8, fontFamily: 'monospace', color: 'var(--color-text-terminal)', marginBottom: 16, minHeight: 60 }}>
        {history.map((h, i) => (
          <div key={i} style={{ marginBottom: 8 }}>
            <div style={{ color: 'var(--color-text-terminal-cmd)' }}>$ {h.command}</div>
            {h.output && <div style={{ whiteSpace: 'pre-wrap', color: 'var(--color-text-terminal-output)' }}>{h.output}</div>}
          </div>
        ))}
        <div style={{ color: 'var(--color-text-terminal-cmd)' }}>{step.prompt}</div>
      </div>

      {/* Context */}
      {step.context ? (
        <Typography.Paragraph>
          <MarkdownRenderer content={step.context} />
        </Typography.Paragraph>
      ) : null}

      {/* Proposals */}
      <Typography.Text strong>Choose the correct command:</Typography.Text>
      <Radio.Group
        onChange={(e) => setSelected(e.target.value)}
        value={selected}
        style={{ width: '100%', marginTop: 8 }}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          {step.proposals.map((p) => (
            <Radio
              key={p.command}
              value={p.command}
              disabled={disabledCommands.has(p.command)}
              style={{
                display: 'block',
                padding: '8px 12px',
                border: '1px solid var(--color-border-input)',
                borderRadius: 6,
                fontFamily: 'monospace',
                ...(disabledCommands.has(p.command)
                  ? { background: 'var(--color-choice-wrong-bg)', textDecoration: 'line-through', borderColor: 'var(--color-choice-wrong-border)' }
                  : {}),
              }}
            >
              {p.command}
            </Radio>
          ))}
        </Space>
      </Radio.Group>

      {feedback && !feedback.is_correct ? (
        <Alert
          message="Incorrect"
          description={feedback.explanation as string}
          type="error"
          showIcon
          style={{ marginTop: 16 }}
        />
      ) : null}

      {feedback?.is_correct ? (
        <Alert message="Correct!" type="success" showIcon style={{ marginTop: 16 }} />
      ) : null}

      <Divider />
      <Button type="primary" onClick={handleSubmit} loading={submitting} disabled={!selected}>
        Submit
      </Button>
    </Card>
  );
};

export default TerminalExercise;
