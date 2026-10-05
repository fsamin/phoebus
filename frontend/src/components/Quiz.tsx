import React, { useState } from 'react';
import { Card, Radio, Checkbox, Input, Button, Alert, Typography, Tag, Space, Divider } from 'antd';
import { CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons';
import MarkdownRenderer from './MarkdownRenderer';

// Shown after submit so that correctness is not conveyed by colour alone
function ChoiceMark({ fb }: { fb?: { correct: boolean; selected: boolean } }) {
  if (fb?.correct) {
    return <span style={{ marginLeft: 8, color: 'var(--color-success-text)' }}><CheckCircleOutlined /> Correct answer</span>;
  }
  if (fb?.selected) {
    return <span style={{ marginLeft: 8, color: 'var(--color-danger-text)' }}><CloseCircleOutlined /> Your answer</span>;
  }
  return null;
}

interface QuizQuestion {
  text: string;
  type: 'multiple-choice' | 'short-answer';
  multi_select?: boolean;
  // correct/pattern/explanation are stripped server-side: the answers only ever
  // come back in the attempt feedback.
  choices?: { text: string; correct?: boolean }[];
  pattern?: string;
  explanation?: string;
}

interface QuizProps {
  questions: QuizQuestion[];
  onSubmit: (body: Record<string, unknown>) => Promise<Record<string, unknown>>;
  onComplete?: () => void;
}

const Quiz: React.FC<QuizProps> = ({ questions, onSubmit, onComplete }) => {
  const [currentIdx, setCurrentIdx] = useState(0);
  const [selected, setSelected] = useState<string[]>([]);
  const [answer, setAnswer] = useState('');
  const [feedback, setFeedback] = useState<Record<string, unknown> | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [results, setResults] = useState<Array<{ idx: number; correct: boolean; explanation?: string }>>([]);
  const [showSummary, setShowSummary] = useState(false);

  if (showSummary) {
    const correct = results.filter((r) => r.correct).length;
    return (
      <Card title="Quiz results">
        <Typography.Title level={4}>
          {correct} of {questions.length} correct
        </Typography.Title>
        {results.map((r) => (
          <div key={r.idx} style={{ marginBottom: 12 }}>
            {r.correct ? (
              <Tag icon={<CheckCircleOutlined />} color="success">Question {r.idx + 1}: correct</Tag>
            ) : (
              <Tag icon={<CloseCircleOutlined />} color="error">Question {r.idx + 1}: incorrect</Tag>
            )}
            <Typography.Text style={{ marginLeft: 8, whiteSpace: 'normal' }}>{questions[r.idx].text}</Typography.Text>
            {r.explanation && (
              <div style={{ marginTop: 4 }}>
                <MarkdownRenderer content={r.explanation} />
              </div>
            )}
          </div>
        ))}
        <Typography.Text type="secondary">To take the quiz again, use Reset Exercise below.</Typography.Text>
      </Card>
    );
  }

  const q = questions[currentIdx];

  const handleSubmit = async () => {
    setSubmitting(true);
    try {
      const body: Record<string, unknown> = {
        question_index: currentIdx,
        type: q.type,
      };
      if (q.type === 'multiple-choice') {
        body.selected = selected;
      } else {
        body.answer = answer;
      }
      const result = await onSubmit(body);
      setFeedback(result);
      setResults((prev) => [...prev, { idx: currentIdx, correct: result.is_correct as boolean, explanation: result.explanation as string | undefined }]);
    } finally {
      setSubmitting(false);
    }
  };

  const handleNext = () => {
    if (currentIdx + 1 >= questions.length) {
      setShowSummary(true);
      onComplete?.();
    } else {
      setCurrentIdx(currentIdx + 1);
      setSelected([]);
      setAnswer('');
      setFeedback(null);
    }
  };

  return (
    <Card
      title={<Tag>Question {currentIdx + 1}/{questions.length}</Tag>}
    >
      <div style={{ marginBottom: 16 }}>
        <MarkdownRenderer content={q.text} />
      </div>
      {q.type === 'multiple-choice' ? (
        q.multi_select ? (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Typography.Text type="secondary">Select all that apply</Typography.Text>
            <Checkbox.Group
              onChange={(values) => setSelected(values as string[])}
              value={selected}
              disabled={!!feedback}
              style={{ width: '100%' }}
            >
              <Space direction="vertical" style={{ width: '100%' }}>
                {q.choices?.map((c) => {
                  const isSelected = selected.includes(c.text);
                  const fb = feedback?.choices_feedback as Array<{ text: string; correct: boolean; selected: boolean }> | undefined;
                  const choiceFb = fb?.find((f) => f.text === c.text);
                  let style: React.CSSProperties = {};
                  if (choiceFb) {
                    style = choiceFb.correct
                      ? { background: 'var(--color-choice-correct-bg)', borderColor: 'var(--color-choice-correct-border)' }
                      : choiceFb.selected
                      ? { background: 'var(--color-choice-wrong-bg)', borderColor: 'var(--color-choice-wrong-border)' }
                      : {};
                  }
                  return (
                    <Checkbox
                      key={c.text}
                      value={c.text}
                      className="quiz-choice"
                      style={{
                        display: 'flex',
                        padding: '8px 12px',
                        border: '1px solid var(--color-border-input)',
                        borderRadius: 6,
                        ...style,
                        ...(isSelected && !feedback ? { borderColor: 'var(--color-choice-selected-border)', background: 'var(--color-choice-selected-bg)' } : {}),
                      }}
                    >
                      {c.text}
                      <ChoiceMark fb={choiceFb} />
                    </Checkbox>
                  );
                })}
              </Space>
            </Checkbox.Group>
          </Space>
        ) : (
          <Radio.Group
            onChange={(e) => setSelected([e.target.value])}
            value={selected[0]}
            disabled={!!feedback}
            style={{ width: '100%' }}
          >
            <Space direction="vertical" style={{ width: '100%' }}>
              {q.choices?.map((c) => {
                const fb = feedback?.choices_feedback as Array<{ text: string; correct: boolean; selected: boolean }> | undefined;
                const choiceFb = fb?.find((f) => f.text === c.text);
                let style: React.CSSProperties = {};
                if (choiceFb) {
                  style = choiceFb.correct
                    ? { background: 'var(--color-choice-correct-bg)', borderColor: 'var(--color-choice-correct-border)' }
                    : choiceFb.selected
                    ? { background: 'var(--color-choice-wrong-bg)', borderColor: 'var(--color-choice-wrong-border)' }
                    : {};
                }
                return (
                  <Radio
                    key={c.text}
                    value={c.text}
                    className="quiz-choice"
                    style={{ display: 'flex', padding: '8px 12px', border: '1px solid var(--color-border-input)', borderRadius: 6, ...style }}
                  >
                    {c.text}
                    <ChoiceMark fb={choiceFb} />
                  </Radio>
                );
              })}
            </Space>
          </Radio.Group>
        )
      ) : (
        <Input
          placeholder="Type your answer..."
          value={answer}
          onChange={(e) => setAnswer(e.target.value)}
          onPressEnter={!feedback ? handleSubmit : undefined}
          disabled={!!feedback}
        />
      )}

      {feedback && (
        <div style={{ marginTop: 16 }}>
          <Alert
            message={feedback.is_correct ? 'Correct.' : q.type === 'multiple-choice' ? 'Incorrect. The correct answer is marked above.' : 'Incorrect.'}
            type={feedback.is_correct ? 'success' : 'error'}
            showIcon
          />
          {(feedback.explanation as string) && (
            <Card size="small" style={{ marginTop: 8, background: 'var(--color-bg-elevated)' }}>
              <MarkdownRenderer content={feedback.explanation as string} />
            </Card>
          )}
        </div>
      )}

      <Divider />
      {!feedback ? (
        <Button
          type="primary"
          onClick={handleSubmit}
          loading={submitting}
          disabled={q.type === 'multiple-choice' ? selected.length === 0 : !answer}
        >
          Check answer
        </Button>
      ) : (
        <Button type="primary" onClick={handleNext}>
          {currentIdx + 1 >= questions.length ? 'See results' : 'Next question'}
        </Button>
      )}
    </Card>
  );
};

export default Quiz;
