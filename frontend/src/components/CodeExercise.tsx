import React, { useState, useMemo, useRef, useCallback } from 'react';
import { Radio, Button, Alert, Typography, Tag, Space, Tree, Tooltip } from 'antd';
import { FileOutlined, FolderOutlined, CheckCircleFilled, BugFilled } from '@ant-design/icons';
import Editor from '@monaco-editor/react';
import MarkdownRenderer from './MarkdownRenderer';
import { useTheme } from '../contexts/ThemeContext';
import type { CodebaseFile } from '../api/client';

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

// Map file extensions to Monaco language IDs
function getLanguage(filePath: string): string {
  const ext = filePath.split('.').pop()?.toLowerCase() || '';
  const map: Record<string, string> = {
    js: 'javascript', jsx: 'javascript', ts: 'typescript', tsx: 'typescript',
    py: 'python', go: 'go', yml: 'yaml', yaml: 'yaml', json: 'json',
    sh: 'shell', bash: 'shell', dockerfile: 'dockerfile', html: 'html',
    htm: 'html', xml: 'xml', css: 'css', sql: 'sql', md: 'markdown',
    rs: 'rust', rb: 'ruby', java: 'java', c: 'c', cpp: 'cpp', h: 'c',
    tf: 'hcl', toml: 'ini', ini: 'ini', makefile: 'makefile',
  };
  return map[ext] || 'plaintext';
}

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
  const { isDark } = useTheme();
  const [selectedFile, setSelectedFile] = useState(target?.file || codebaseFiles[0]?.file_path || '');
  const [selectedLines, setSelectedLines] = useState<number[]>([]);
  const [phase, setPhase] = useState<'identify' | 'fix'>(mode === 'B' ? 'fix' : 'identify');
  const [selectedPatch, setSelectedPatch] = useState('');
  const [feedback, setFeedback] = useState<Record<string, unknown> | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [completed, setCompleted] = useState(false);
  const [bottomPanelHeight, setBottomPanelHeight] = useState(200);
  const editorRef = useRef<unknown>(null);
  const decorationsRef = useRef<any>(null);
  const resizingRef = useRef(false);

  const currentFile = codebaseFiles.find((f) => f.file_path === selectedFile);
  const treeData = useMemo(() => buildTreeData(codebaseFiles), [codebaseFiles]);

  // Update editor decorations when selected lines change
  const updateDecorations = useCallback((editor: any) => {
    if (!editor) return;
    const newDecorations = selectedLines.map((lineNum) => ({
      range: { startLineNumber: lineNum, startColumn: 1, endLineNumber: lineNum, endColumn: 1 },
      options: {
        isWholeLine: true,
        className: 'line-selected',
        glyphMarginClassName: 'line-glyph-selected',
      },
    }));
    if (phase === 'fix' && target?.file === selectedFile) {
      target.lines.forEach((lineNum) => {
        newDecorations.push({
          range: { startLineNumber: lineNum, startColumn: 1, endLineNumber: lineNum, endColumn: 1 },
          options: {
            isWholeLine: true,
            className: 'line-target',
            glyphMarginClassName: 'line-glyph-target',
          },
        });
      });
    }
    if (decorationsRef.current) {
      decorationsRef.current.clear();
    }
    decorationsRef.current = editor.createDecorationsCollection(newDecorations);
  }, [selectedLines, phase, target, selectedFile]);

  const handleEditorMount = (editor: any) => {
    editorRef.current = editor;
    editor.updateOptions({ readOnly: true, glyphMargin: phase === 'identify' });
    editor.onMouseDown((e: any) => {
      if (phase !== 'identify' || feedback) return;
      const lineNum = e.target?.position?.lineNumber;
      if (!lineNum) return;
      if (e.target.type === 2 || e.target.type === 3 || e.target.type === 4) {
        setSelectedLines((prev) =>
          prev.includes(lineNum) ? prev.filter((l) => l !== lineNum) : [...prev, lineNum]
        );
      }
    });
    updateDecorations(editor);
  };

  useMemo(() => {
    if (editorRef.current) updateDecorations(editorRef.current);
  }, [selectedLines, updateDecorations]);

  const handleSubmitIdentify = async () => {
    setSubmitting(true);
    try {
      const result = await onSubmit({ phase: 'identify', selected_lines: selectedLines });
      setFeedback(result);
      if (result.is_correct) {
        setTimeout(() => { setPhase('fix'); setFeedback(null); }, 1500);
      }
    } finally { setSubmitting(false); }
  };

  const handleSubmitFix = async () => {
    setSubmitting(true);
    try {
      const result = await onSubmit({ phase: 'fix', selected_patch: selectedPatch });
      setFeedback(result);
      if (result.is_correct) setCompleted(true);
    } finally { setSubmitting(false); }
  };

  const handleResizeStart = (e: React.MouseEvent) => {
    e.preventDefault();
    resizingRef.current = true;
    const startY = e.clientY;
    const startHeight = bottomPanelHeight;
    const onMove = (ev: MouseEvent) => {
      if (!resizingRef.current) return;
      setBottomPanelHeight(Math.max(100, Math.min(500, startHeight + (startY - ev.clientY))));
    };
    const onUp = () => {
      resizingRef.current = false;
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: 'calc(100vh - 64px)', background: 'var(--color-bg-ide)' }}>
      <style>{`
        .line-selected { background: var(--color-ide-line-selected) !important; }
        .line-glyph-selected { background: var(--color-ide-glyph-selected); border-radius: 50%; margin-left: 4px; width: 8px !important; height: 8px !important; margin-top: 6px; }
        .line-target { background: var(--color-ide-line-target) !important; }
        .line-glyph-target { background: var(--color-ide-glyph-target); border-radius: 50%; margin-left: 4px; width: 8px !important; height: 8px !important; margin-top: 6px; }
        .ide-bottom-panel .ant-btn-primary:disabled { background: var(--color-ide-disabled-bg) !important; color: var(--color-ide-disabled-text) !important; border-color: var(--color-ide-disabled-border) !important; }
      `}</style>

      {/* Top bar */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '0 12px', height: 36, background: 'var(--color-bg-ide-secondary)', borderBottom: '1px solid var(--color-border-ide)',
        flexShrink: 0,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <FileOutlined style={{ color: 'var(--color-text-ide)', fontSize: 13 }} />
          <Typography.Text style={{ color: 'var(--color-text-ide)', fontSize: 13 }}>{selectedFile}</Typography.Text>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {completed && <Tag color="success" icon={<CheckCircleFilled />}>Completed</Tag>}
          {!completed && mode !== 'B' && (
            <Tag color={phase === 'identify' ? 'processing' : 'warning'} icon={<BugFilled />}>
              {phase === 'identify' ? 'Phase 1: Find the bug' : 'Phase 2: Select the fix'}
            </Tag>
          )}
          {!completed && mode === 'B' && <Tag color="warning">Select the fix</Tag>}
        </div>
      </div>

      {/* Main area */}
      <div style={{ display: 'flex', flex: 1, minHeight: 0 }}>
        {/* File explorer */}
        <div style={{
          width: 200, background: 'var(--color-bg-ide-secondary)', borderRight: '1px solid var(--color-border-ide)',
          overflow: 'auto', flexShrink: 0, padding: '8px 0',
        }}>
          <div style={{ padding: '4px 12px 8px', color: 'var(--color-text-ide-secondary)', fontSize: 11, textTransform: 'uppercase', letterSpacing: 1 }}>
            Explorer
          </div>
          <Tree
            treeData={treeData}
            selectedKeys={[selectedFile]}
            onSelect={(keys) => keys[0] && setSelectedFile(keys[0] as string)}
            defaultExpandAll
            style={{ background: 'transparent', color: 'var(--color-text-ide-explorer)' }}
            className="ide-tree"
          />
          <style>{`
            .ide-tree, .ide-tree * { background-color: transparent !important; }
            .ide-tree .ant-tree-node-content-wrapper { color: var(--color-text-ide-explorer) !important; }
            .ide-tree .ant-tree-node-content-wrapper:hover { background-color: var(--color-ide-tree-hover) !important; }
            .ide-tree .ant-tree-node-selected .ant-tree-node-content-wrapper,
            .ide-tree .ant-tree-node-content-wrapper.ant-tree-node-selected { background-color: var(--color-ide-tree-selected) !important; color: var(--color-text-primary) !important; }
            .ide-tree .ant-tree-switcher { color: var(--color-text-ide-explorer) !important; }
            .ide-tree .ant-tree-indent-unit { width: 16px; }
          `}</style>
        </div>

        {/* Monaco editor */}
        <div style={{ flex: 1, minWidth: 0 }}>
          <Editor
            height="100%"
            language={getLanguage(selectedFile)}
            value={currentFile?.content || '// No file selected'}
            theme={isDark ? 'vs-dark' : 'vs'}
            onMount={handleEditorMount}
            options={{
              readOnly: true,
              minimap: { enabled: true, scale: 2, size: 'proportional' },
              fontSize: 14,
              lineNumbers: 'on',
              scrollBeyondLastLine: false,
              glyphMargin: phase === 'identify',
              folding: true,
              renderLineHighlight: 'line',
              wordWrap: 'off',
              automaticLayout: true,
              contextmenu: false,
              cursorStyle: 'line',
            }}
          />
        </div>
      </div>

      {/* Resize handle */}
      <div
        onMouseDown={handleResizeStart}
        style={{
          height: 4, background: 'var(--color-border-resize)', cursor: 'ns-resize', flexShrink: 0,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}
      >
        <div style={{ width: 40, height: 2, background: 'var(--color-text-secondary)', borderRadius: 1 }} />
      </div>

      {/* Bottom panel */}
      <div className="ide-bottom-panel" style={{
        height: bottomPanelHeight, background: 'var(--color-bg-ide-panel)', borderTop: '1px solid var(--color-border-ide)',
        overflow: 'auto', flexShrink: 0, padding: '12px 16px', color: 'var(--color-text-ide)',
      }}>
        {completed ? (
          <div>
            <Alert message="Exercise complete!" type="success" showIcon style={{ marginBottom: 8 }} />
            {typeof feedback?.explanation === 'string' && (
              <div style={{ background: 'var(--color-bg-ide-secondary)', padding: 12, borderRadius: 4 }}>
                <MarkdownRenderer content={feedback.explanation} />
              </div>
            )}
          </div>
        ) : phase === 'identify' ? (
          <div>
            <div style={{ marginBottom: 8 }}>
              <MarkdownRenderer content={description} />
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <Typography.Text style={{ color: 'var(--color-text-ide)' }}>
                Click line numbers to select problematic lines.
                Selected: <strong>{selectedLines.length > 0 ? selectedLines.sort((a, b) => a - b).join(', ') : 'none'}</strong>
              </Typography.Text>
              <Button type="primary" size="small" onClick={handleSubmitIdentify} loading={submitting} disabled={selectedLines.length === 0}>
                Validate
              </Button>
            </div>
            {feedback && !feedback.is_correct && (
              <Alert message={`${feedback.matched}/${feedback.total} lines found`} description={feedback.hint as string} type="warning" showIcon style={{ marginTop: 8 }} />
            )}
            {feedback && (feedback.is_correct as boolean) && (
              <Alert message="Correct! Moving to fix phase..." type="success" showIcon style={{ marginTop: 8 }} />
            )}
          </div>
        ) : (
          <div>
            <div style={{ marginBottom: 8 }}>
              <Typography.Text strong style={{ color: 'var(--color-text-ide)' }}>Select the correct fix:</Typography.Text>
            </div>
            <Radio.Group onChange={(e) => setSelectedPatch(e.target.value)} value={selectedPatch} style={{ width: '100%' }} disabled={completed}>
              <Space direction="vertical" style={{ width: '100%' }}>
                {patches.map((p) => (
                  <Radio key={p.label} value={p.label} style={{ color: 'var(--color-text-ide)' }}>
                    <Tooltip title={p.diff} placement="topLeft" overlayStyle={{ maxWidth: 500 }} overlayInnerStyle={{ whiteSpace: 'pre', fontFamily: 'monospace', fontSize: 12 }}>
                      <Typography.Text style={{ color: 'var(--color-text-ide)' }}>{p.label}</Typography.Text>
                    </Tooltip>
                  </Radio>
                ))}
              </Space>
            </Radio.Group>
            <div style={{ marginTop: 8 }}>
              <Button type="primary" size="small" onClick={handleSubmitFix} loading={submitting} disabled={!selectedPatch || completed}>
                Submit Fix
              </Button>
            </div>
            {feedback && (
              <Alert
                message={feedback.is_correct ? 'Correct!' : 'Incorrect'}
                description={feedback.explanation as string}
                type={feedback.is_correct ? 'success' : 'error'}
                showIcon style={{ marginTop: 8 }}
              />
            )}
          </div>
        )}
      </div>
    </div>
  );
};

export default CodeExercise;
