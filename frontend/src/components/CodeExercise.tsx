import React, { useState, useMemo } from 'react';
import { Card, Radio, Button, Alert, Typography, Tag, Space, Divider, Tree } from 'antd';
import { FileOutlined, FolderOutlined } from '@ant-design/icons';
import MarkdownRenderer from './MarkdownRenderer';
import type { CodebaseFile } from '../api/client';
import hljs from 'highlight.js/lib/core';
import javascript from 'highlight.js/lib/languages/javascript';
import typescript from 'highlight.js/lib/languages/typescript';
import python from 'highlight.js/lib/languages/python';
import go from 'highlight.js/lib/languages/go';
import yaml from 'highlight.js/lib/languages/yaml';
import json from 'highlight.js/lib/languages/json';
import bash from 'highlight.js/lib/languages/bash';
import dockerfile from 'highlight.js/lib/languages/dockerfile';
import xml from 'highlight.js/lib/languages/xml';
import css from 'highlight.js/lib/languages/css';
import sql from 'highlight.js/lib/languages/sql';
import markdown from 'highlight.js/lib/languages/markdown';
import 'highlight.js/styles/github.css';

hljs.registerLanguage('javascript', javascript);
hljs.registerLanguage('typescript', typescript);
hljs.registerLanguage('python', python);
hljs.registerLanguage('go', go);
hljs.registerLanguage('yaml', yaml);
hljs.registerLanguage('json', json);
hljs.registerLanguage('bash', bash);
hljs.registerLanguage('dockerfile', dockerfile);
hljs.registerLanguage('xml', xml);
hljs.registerLanguage('html', xml);
hljs.registerLanguage('css', css);
hljs.registerLanguage('sql', sql);
hljs.registerLanguage('markdown', markdown);

interface Patch {
  label: string;
  correct: boolean;
  explanation: string;
  diff: string;
}

interface CodeExerciseProps {
  mode: string;
  description: string;
  target?: { file: string; lines: number[] };
  patches: Patch[];
  codebaseFiles: CodebaseFile[];
  onSubmit: (body: Record<string, unknown>) => Promise<Record<string, unknown>>;
}

// Build tree data from flat file paths
function buildTreeData(files: CodebaseFile[]) {
  const root: Record<string, unknown>[] = [];
  const dirs: Record<string, Record<string, unknown>> = {};

  for (const f of files) {
    const parts = f.file_path.split('/');
    if (parts.length === 1) {
      root.push({ title: parts[0], key: f.file_path, icon: <FileOutlined />, isLeaf: true });
    } else {
      const dirKey = parts.slice(0, -1).join('/');
      if (!dirs[dirKey]) {
        dirs[dirKey] = { title: dirKey, key: dirKey, icon: <FolderOutlined />, children: [] };
        root.push(dirs[dirKey]);
      }
      (dirs[dirKey].children as Record<string, unknown>[]).push({
        title: parts[parts.length - 1],
        key: f.file_path,
        icon: <FileOutlined />,
        isLeaf: true,
      });
    }
  }
  return root;
}

const CodeExercise: React.FC<CodeExerciseProps> = ({ mode, description, target, patches, codebaseFiles, onSubmit }) => {
  const [selectedFile, setSelectedFile] = useState(target?.file || codebaseFiles[0]?.file_path || '');
  const [selectedLines, setSelectedLines] = useState<number[]>([]);
  const [phase, setPhase] = useState<'identify' | 'fix'>(mode === 'B' ? 'fix' : 'identify');
  const [selectedPatch, setSelectedPatch] = useState('');
  const [feedback, setFeedback] = useState<Record<string, unknown> | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [completed, setCompleted] = useState(false);

  const currentFile = codebaseFiles.find((f) => f.file_path === selectedFile);
  const lines = currentFile?.content.split('\n') || [];
  const treeData = buildTreeData(codebaseFiles);

  // Syntax highlighting
  const highlightedLines = useMemo(() => {
    if (!currentFile) return [];
    const ext = selectedFile.split('.').pop()?.toLowerCase() || '';
    const langMap: Record<string, string> = {
      js: 'javascript', jsx: 'javascript', ts: 'typescript', tsx: 'typescript',
      py: 'python', go: 'go', yml: 'yaml', yaml: 'yaml', json: 'json',
      sh: 'bash', bash: 'bash', dockerfile: 'dockerfile', html: 'html',
      htm: 'html', xml: 'xml', css: 'css', sql: 'sql', md: 'markdown',
    };
    const lang = langMap[ext];
    if (!lang) return lines.map((l) => l);
    try {
      const result = hljs.highlight(currentFile.content, { language: lang });
      return result.value.split('\n');
    } catch {
      return lines.map((l) => l);
    }
  }, [currentFile, selectedFile, lines]);

  const toggleLine = (lineNum: number) => {
    if (phase !== 'identify' || feedback) return;
    setSelectedLines((prev) =>
      prev.includes(lineNum) ? prev.filter((l) => l !== lineNum) : [...prev, lineNum]
    );
  };

  const handleSubmitIdentify = async () => {
    setSubmitting(true);
    try {
      const result = await onSubmit({ phase: 'identify', selected_lines: selectedLines });
      setFeedback(result);
      if (result.is_correct) {
        setTimeout(() => {
          setPhase('fix');
          setFeedback(null);
        }, 1500);
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleSubmitFix = async () => {
    setSubmitting(true);
    try {
      const result = await onSubmit({ phase: 'fix', selected_patch: selectedPatch });
      setFeedback(result);
      if (result.is_correct) {
        setCompleted(true);
      }
    } finally {
      setSubmitting(false);
    }
  };

  if (completed) {
    return (
      <Card title="Exercise Complete ✅">
        <Alert message="Well done!" type="success" showIcon />
        {typeof feedback?.explanation === 'string' && (
          <Card size="small" style={{ marginTop: 12, background: '#fafafa' }}>
            <MarkdownRenderer content={feedback.explanation} />
          </Card>
        )}
      </Card>
    );
  }

  return (
    <Card
      title={
        <span>
          Code Exercise
          {mode !== 'B' && <Tag style={{ marginLeft: 12 }}>Phase {phase === 'identify' ? '1: Identify' : '2: Fix'}</Tag>}
        </span>
      }
    >
      <div style={{ display: 'flex', gap: 16, marginBottom: 16 }}>
        {/* File tree */}
        <div style={{ width: 200, borderRight: '1px solid #f0f0f0', paddingRight: 16 }}>
          <Tree
            treeData={treeData}
            selectedKeys={[selectedFile]}
            onSelect={(keys) => keys[0] && setSelectedFile(keys[0] as string)}
            defaultExpandAll
          />
        </div>

        {/* Code viewer */}
        <div style={{ flex: 1, overflow: 'auto', maxHeight: 400 }}>
          <pre style={{ margin: 0, padding: 16, background: '#f6f8fa', borderRadius: 6, fontSize: 13, lineHeight: '20px' }}>
            {lines.map((_line, i) => {
              const lineNum = i + 1;
              const isSelected = selectedLines.includes(lineNum);
              const isTarget = target?.file === selectedFile && target?.lines.includes(lineNum);
              return (
                <div
                  key={i}
                  onClick={() => toggleLine(lineNum)}
                  style={{
                    cursor: phase === 'identify' && !feedback ? 'pointer' : 'default',
                    background: isSelected
                      ? '#e6f7ff'
                      : (phase === 'fix' && isTarget)
                      ? '#f6ffed'
                      : 'transparent',
                    display: 'flex',
                    borderLeft: isSelected ? '3px solid #1890ff' : isTarget && phase === 'fix' ? '3px solid #52c41a' : '3px solid transparent',
                  }}
                >
                  <span style={{ width: 40, textAlign: 'right', paddingRight: 12, color: '#999', userSelect: 'none' }}>
                    {lineNum}
                  </span>
                  <code dangerouslySetInnerHTML={{ __html: highlightedLines[i] || '' }} />
                </div>
              );
            })}
          </pre>
        </div>
      </div>

      {/* Description */}
      <Card size="small" style={{ marginBottom: 16, background: '#fafafa' }}>
        <MarkdownRenderer content={description} />
      </Card>

      {phase === 'identify' && (
        <>
          <Typography.Text>
            Selected lines: {selectedLines.sort((a, b) => a - b).join(', ') || 'none'}
          </Typography.Text>

          {feedback && !feedback.is_correct && (
            <Alert
              message={`${(feedback as Record<string, unknown>).matched}/${(feedback as Record<string, unknown>).total} lines found`}
              description={(feedback as Record<string, unknown>).hint as string}
              type="warning"
              showIcon
              style={{ marginTop: 12 }}
            />
          )}
          {feedback?.is_correct && (
            <Alert message="Correct! Moving to fix phase..." type="success" showIcon style={{ marginTop: 12 }} />
          )}

          <Divider />
          <Button type="primary" onClick={handleSubmitIdentify} loading={submitting} disabled={selectedLines.length === 0}>
            Validate Selection
          </Button>
        </>
      )}

      {phase === 'fix' && (
        <>
          <Typography.Text strong>Select the correct fix:</Typography.Text>
          <Radio.Group
            onChange={(e) => setSelectedPatch(e.target.value)}
            value={selectedPatch}
            style={{ width: '100%', marginTop: 8 }}
            disabled={completed}
          >
            <Space direction="vertical" style={{ width: '100%' }}>
              {patches.map((p) => (
                <Radio key={p.label} value={p.label} style={{ display: 'block', padding: '8px 0' }}>
                  <div>
                    <Typography.Text strong>{p.label}</Typography.Text>
                    <pre style={{ margin: '8px 0', padding: 12, background: '#f6f8fa', borderRadius: 6, fontSize: 12, whiteSpace: 'pre-wrap' }}>
                      {p.diff}
                    </pre>
                  </div>
                </Radio>
              ))}
            </Space>
          </Radio.Group>

          {feedback && (
            <Alert
              message={feedback.is_correct ? 'Correct!' : 'Incorrect'}
              description={feedback.explanation as string}
              type={feedback.is_correct ? 'success' : 'error'}
              showIcon
              style={{ marginTop: 16 }}
            />
          )}

          <Divider />
          <Button type="primary" onClick={handleSubmitFix} loading={submitting} disabled={!selectedPatch || completed}>
            Submit Fix
          </Button>
        </>
      )}
    </Card>
  );
};

export default CodeExercise;
