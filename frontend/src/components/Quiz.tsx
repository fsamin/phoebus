import React, { useState } from 'react';
import { Card, Radio, Input, Button, Alert, Typography, Tag, Space, Divider } from 'antd';
import { CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons';
import MarkdownRenderer from './MarkdownRenderer';

interface QuizQuestion {
  text: string;
  type: 'multiple-choice' | 'short-answer';
  multi_select?: boolean;
  choices?: { text: string; correct: boolean }[];
  pattern?: string;
  explanation?: string;
}

interface QuizProps {
  questions: QuizQuestion[];
  onSubmit: (body: Record<string, unknown>) => Promise<Record<string, unknown>>;
}

const Quiz: React.FC<QuizProps> = ({ questions, onSubmit }) => {
  const [currentIdx, setCurrentIdx] = useState(0);
  const [selected, setSelected] = useState<string[]>([]);
  const [answer, setAnswer] = useState('');
  const [feedback, setFeedback] = useState<Record<string, unknown> | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [results, setResults] = useState<Array<{ idx: number; correct: boolean }>>([]);
  const [showSummary, setShowSummary] = useState(false);

  if (showSummary) {
    const correct = results.filter((r) => r.correct).length;
    return (
      <Card title="Quiz Complete">
        <Typography.Title level={4}>
          {correct}/{questions.length} correct
        </Typography.Title>
        {results.map((r) => (
          <div key={r.idx} style={{ marginBottom: 8 }}>
            {r.correct ? (
              <Tag icon={<CheckCircleOutlined />} color="success">Q{r.idx + 1}: Correct</Tag>
            ) : (
              <Tag icon={<CloseCircleOutlined />} color="error">Q{r.idx + 1}: Incorrect</Tag>
            )}
            <Typography.Text style={{ marginLeft: 8 }}>{questions[r.idx].text}</Typography.Text>
          </div>
        ))}
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
      setResults((prev) => [...prev, { idx: currentIdx, correct: result.is_correct as boolean }]);
    } finally {
      setSubmitting(false);
    }
  };

  const handleNext = () => {
    if (currentIdx + 1 >= questions.length) {
      setShowSummary(true);
    } else {
      setCurrentIdx(currentIdx + 1);
      setSelected([]);
      setAnswer('');
      setFeedback(null);
    }
  };

  return (
    <Card
      title={
        <span>
          {q.text}
          <Tag style={{ marginLeft: 12 }}>Q {currentIdx + 1}/{questions.length}</Tag>
        </span>
      }
    >
      {q.type === 'multiple-choice' ? (
        q.multi_select ? (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Typography.Text type="secondary">Select all that apply</Typography.Text>
            {q.choices?.map((c) => {
              const isSelected = selected.includes(c.text);
              const fb = feedback?.choices_feedback as Array<{ text: string; correct: boolean; selected: boolean }> | undefined;
              const choiceFb = fb?.find((f) => f.text === c.text);
              let style: React.CSSProperties = {};
              if (choiceFb) {
                style = choiceFb.correct
                  ? { background: '#f6ffed', borderColor: '#52c41a' }
                  : choiceFb.selected
                  ? { background: '#fff2f0', borderColor: '#ff4d4f' }
                  : {};
              }
              return (
                <div
                  key={c.text}
                  style={{
                    padding: '8px 12px',
                    border: '1px solid #d9d9d9',
                    borderRadius: 6,
                    cursor: feedback ? 'default' : 'pointer',
                    ...style,
                    ...(isSelected && !feedback ? { borderColor: '#1890ff', background: '#e6f7ff' } : {}),
                  }}
                  onClick={() => {
                    if (feedback) return;
                    setSelected((prev) =>
                      prev.includes(c.text) ? prev.filter((s) => s !== c.text) : [...prev, c.text]
                    );
                  }}
                >
                  {c.text}
                </div>
              );
            })}
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
                    ? { background: '#f6ffed', borderColor: '#52c41a' }
                    : choiceFb.selected
                    ? { background: '#fff2f0', borderColor: '#ff4d4f' }
                    : {};
                }
                return (
                  <Radio
                    key={c.text}
                    value={c.text}
                    style={{ display: 'block', padding: '8px 12px', border: '1px solid #d9d9d9', borderRadius: 6, ...style }}
                  >
                    {c.text}
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
            message={feedback.is_correct ? 'Correct!' : 'Incorrect'}
            type={feedback.is_correct ? 'success' : 'error'}
            showIcon
          />
          {(feedback.explanation as string) && (
            <Card size="small" style={{ marginTop: 8, background: '#fafafa' }}>
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
          Submit
        </Button>
      ) : (
        <Button type="primary" onClick={handleNext}>
          {currentIdx + 1 >= questions.length ? 'See Results' : 'Next Question'}
        </Button>
      )}
    </Card>
  );
};

export default Quiz;
