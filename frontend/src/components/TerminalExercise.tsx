import React, { useState, useRef, useEffect } from 'react';
import { Typography, Tag } from 'antd';
import { CheckCircleFilled, CloseCircleFilled, LoadingOutlined } from '@ant-design/icons';
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
  const terminalEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    terminalEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [history, feedback, currentStep, selected]);

  const step = completed ? null : steps[currentStep];

  const handleSubmit = async () => {
    if (!selected || submitting) return;
    setSubmitting(true);
    try {
      const result = await onSubmit({ step_number: currentStep + 1, selected_command: selected });
      setFeedback(result);
      if (result.is_correct) {
        setTimeout(() => {
          setHistory((prev) => [...prev, { command: selected, output: (result.output as string) || '' }]);
          setFeedback(null);
          if (currentStep + 1 >= steps.length) {
            setCompleted(true);
          } else {
            setCurrentStep(currentStep + 1);
            setSelected('');
            setDisabledCommands(new Set());
          }
        }, 1200);
      } else {
        setDisabledCommands((prev) => new Set([...prev, selected]));
        setSelected('');
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && selected && !submitting) {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <div style={{ maxWidth: 900 }}>
      {/* Context above terminal */}
      {introduction && currentStep === 0 && !completed && (
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          <MarkdownRenderer content={introduction} />
        </Typography.Paragraph>
      )}
      {step?.context && (
        <Typography.Paragraph style={{ marginBottom: 12 }}>
          <MarkdownRenderer content={step.context} />
        </Typography.Paragraph>
      )}

      {/* Step counter */}
      {!completed && (
        <div style={{ marginBottom: 8, display: 'flex', alignItems: 'center', gap: 8 }}>
          <Tag color="var(--color-primary)">Step {currentStep + 1}/{steps.length}</Tag>
        </div>
      )}

      {/* Terminal */}
      <div
        onKeyDown={handleKeyDown}
        tabIndex={0}
        style={{
          background: 'var(--color-bg-terminal)',
          borderRadius: 10,
          fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Consolas', monospace",
          fontSize: 14,
          lineHeight: 1.7,
          overflow: 'hidden',
          border: '1px solid rgba(255,255,255,0.06)',
          boxShadow: '0 4px 24px rgba(0,0,0,0.3)',
        }}
      >
        {/* Title bar */}
        <div style={{
          background: 'rgba(255,255,255,0.05)',
          padding: '8px 16px',
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          borderBottom: '1px solid rgba(255,255,255,0.06)',
        }}>
          <span style={{ width: 12, height: 12, borderRadius: '50%', background: '#ff5f57', display: 'inline-block' }} />
          <span style={{ width: 12, height: 12, borderRadius: '50%', background: '#febc2e', display: 'inline-block' }} />
          <span style={{ width: 12, height: 12, borderRadius: '50%', background: '#28c840', display: 'inline-block' }} />
          <span style={{ marginLeft: 'auto', color: 'rgba(255,255,255,0.35)', fontSize: 12 }}>
            {completed ? 'Exercise Complete' : 'Terminal Exercise'}
          </span>
        </div>

        {/* Terminal body */}
        <div style={{ padding: '16px 20px', maxHeight: 500, overflowY: 'auto' }}>
          {/* Command history */}
          {history.map((h, i) => (
            <div key={i} style={{ marginBottom: 6 }}>
              <div>
                <span style={{ color: '#6ec6ff' }}>~</span>
                <span style={{ color: 'rgba(255,255,255,0.35)' }}> › </span>
                <span style={{ color: 'var(--color-text-terminal-cmd)' }}>{h.command}</span>
              </div>
              {h.output && (
                <div style={{ whiteSpace: 'pre-wrap', color: 'var(--color-text-terminal-output)', paddingLeft: 4 }}>{h.output}</div>
              )}
            </div>
          ))}

          {/* Completed state */}
          {completed && (
            <div style={{ marginTop: 8 }}>
              <div style={{ color: '#28c840', display: 'flex', alignItems: 'center', gap: 8 }}>
                <CheckCircleFilled /> All steps completed successfully!
              </div>
            </div>
          )}

          {/* Active prompt + suggestions */}
          {step && (
            <>
              {/* Current prompt line */}
              <div style={{ marginTop: history.length > 0 ? 4 : 0 }}>
                <span style={{ color: '#6ec6ff' }}>~</span>
                <span style={{ color: 'rgba(255,255,255,0.35)' }}> › </span>
                {selected ? (
                  <span style={{ color: 'var(--color-text-terminal-cmd)' }}>{selected}</span>
                ) : (
                  <span className="terminal-cursor" style={{ color: 'rgba(255,255,255,0.6)' }}>▌</span>
                )}
              </div>

              {/* Feedback inline */}
              {feedback && !feedback.is_correct && (
                <div style={{ color: '#ff6b6b', padding: '6px 0 2px 4px', display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                  <CloseCircleFilled style={{ marginTop: 3 }} />
                  <span>{feedback.explanation as string}</span>
                </div>
              )}
              {feedback?.is_correct && (
                <div style={{ color: '#28c840', padding: '6px 0 2px 4px', display: 'flex', alignItems: 'center', gap: 8 }}>
                  <CheckCircleFilled />
                  <span>Correct!</span>
                </div>
              )}

              {/* Submitting indicator */}
              {submitting && (
                <div style={{ color: 'var(--color-primary)', padding: '6px 0 2px 4px', display: 'flex', alignItems: 'center', gap: 8 }}>
                  <LoadingOutlined spin />
                  <span>Executing...</span>
                </div>
              )}

              {/* Command suggestions */}
              {!feedback?.is_correct && !submitting && (
                <div style={{ marginTop: 10, borderTop: '1px solid rgba(255,255,255,0.08)', paddingTop: 10 }}>
                  <div style={{ color: 'rgba(255,255,255,0.35)', fontSize: 12, marginBottom: 6 }}>
                    Select a command:
                  </div>
                  {step.proposals.map((p) => {
                    const isDisabled = disabledCommands.has(p.command);
                    const isSelected = selected === p.command;
                    return (
                      <div
                        key={p.command}
                        onClick={() => { if (!isDisabled) setSelected(p.command); }}
                        style={{
                          padding: '6px 12px',
                          marginBottom: 3,
                          borderRadius: 4,
                          cursor: isDisabled ? 'not-allowed' : 'pointer',
                          display: 'flex',
                          alignItems: 'center',
                          gap: 10,
                          transition: 'background 0.15s',
                          background: isSelected
                            ? 'rgba(110, 198, 255, 0.12)'
                            : 'transparent',
                          ...(isDisabled ? { opacity: 0.35 } : {}),
                        }}
                        onMouseEnter={(e) => {
                          if (!isDisabled && !isSelected) e.currentTarget.style.background = 'rgba(255,255,255,0.05)';
                        }}
                        onMouseLeave={(e) => {
                          if (!isDisabled && !isSelected) e.currentTarget.style.background = 'transparent';
                        }}
                      >
                        <span style={{
                          color: isDisabled ? '#ff6b6b' : isSelected ? '#6ec6ff' : '#80d8ff',
                          textDecoration: isDisabled ? 'line-through' : 'none',
                          flex: 1,
                        }}>
                          $ {p.command}
                        </span>
                        {isSelected && (
                          <span style={{ color: 'rgba(255,255,255,0.35)', fontSize: 12 }}>
                            press Enter ⏎
                          </span>
                        )}
                        {isDisabled && (
                          <CloseCircleFilled style={{ color: '#ff6b6b', fontSize: 12 }} />
                        )}
                      </div>
                    );
                  })}
                </div>
              )}
            </>
          )}
          <div ref={terminalEndRef} />
        </div>
      </div>
    </div>
  );
};

export default TerminalExercise;
