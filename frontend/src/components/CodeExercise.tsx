import React, { useState, useMemo, useRef, useCallback } from 'react';
import { Radio, Button, Alert, Typography, Tag, Space, Tree, Tooltip } from 'antd';
import { FileOutlined, FolderOutlined, CheckCircleFilled, BugFilled } from '@ant-design/icons';
import Editor from '@monaco-editor/react';
import MarkdownRenderer from './MarkdownRenderer';
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
  const [selectedFile, setSelectedFile] = useState(target?.file || codebaseFiles[0]?.file_path || '');
  const [selectedLines, setSelectedLines] = useState<number[]>([]);
  const [phase, setPhase] = useState<'identify' | 'fix'>(mode === 'B' ? 'fix' : 'identify');
  const [selectedPatch, setSelectedPatch] = useState('');
  const [feedback, setFeedback] = useState<Record<string, unknown> | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [completed, setCompleted] = useState(false);
  const [bottomPanelHeight, setBottomPanelHeight] = useState(200);
  const editorRef = useRef<unknown>(null);
  const decorationsRef = useRef<string[]>([]);
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
    // Add target line decorations in fix phase
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
    decorationsRef.current = editor.deltaDecorations(decorationsRef.current, newDecorations);
  }, [selectedLines, phase, target, selectedFile]);

  const handleEditorMount = (editor: any) => {
    editorRef.current = editor;
    editor.updateOptions({ readOnly: true, glyphMargin: phase === 'identify' });

    // Click on gutter to toggle line selection (identify phase)
    editor.onMouseDown((e: any) => {
      if (phase !== 'identify' || feedback) return;
      const lineNum = e.target?.position?.lineNumber;
      if (!lineNum) return;
      // Allow click on line number or glyph margin
      if (e.target.type === 2 || e.target.type === 3 || e.target.type === 4) {
        setSelectedLines((prev) =>
          prev.includes(lineNum) ? prev.filter((l) => l !== lineNum) : [...prev, lineNum]
        );
      }
    });

    updateDecorations(editor);
  };

  // Update decorations when selectedLines change
  useMemo(() => {
    if (editorRef.current) updateDecorations(editorRef.current);
  }, [selectedLines, updateDecorations]);

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
      if (result.is_correct) setCompleted(true);
    } finally {
      setSubmitting(false);
    }
  };

  // Resize handle for bottom panel
  const handleResizeStart = (e: React.MouseEvent) => {
    e.preventDefault();
    resizingRef.current = true;
    const startY = e.clientY;
    const startHeight = bottomPanelHeight;
    const onMove = (ev: MouseEvent) => {
      if (!resizingRef.current) return;
      const newH = Math.max(100, Math.min(500, startHeight + (startY - ev.clientY)));
      setBottomPanelHeight(newH);
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
    <div style={{ display: 'flex', flexDirection: 'column', height: 'calc(100vh - 64px)', margin: '-24px', background: '#1e1e1e' }}>
      {/* Injected styles for editor decorations */}
      <style>{`
        .line-selected { background: rgba(30, 136, 229, 0.15) !important; }
        .line-glyph-selected { background: #1890ff; border-radius: 50%; margin-left: 4px; width: 8px !important; height: 8px !important; margin-top: 6px; }
        .line-target { background: rgba(82, 196, 26, 0.12) !important; }
        .line-glyph-target { background: #52c41a; border-radius: 50%; margin-left: 4px; width: 8px !important; height: 8px !important; margin-top: 6px; }
      `}</style>

      {/* Top bar: file tabs + phase indicator */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '0 12px', height: 36, background: '#252526', borderBottom: '1px solid #3c3c3c',
        flexShrink: 0,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <FileOutlined style={{ color: '#cccccc', fontSize: 13 }} />
          <Typography.Text style={{ color: '#cccccc', fontSize: 13 }}>{selectedFile}</Typography.Text>
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

      {/* Main area: sidebar + editor */}
      <div style={{ display: 'flex', flex: 1, minHeight: 0 }}>
        {/* File explorer */}
        <div style={{
          width: 200, background: '#252526', borderRight: '1px solid #3c3c3c',
          overflow: 'auto', flexShrink: 0, padding: '8px 0',
        }}>
          <div style={{ padding: '4px 12px 8px', color: '#bbbbbb', fontSize: 11, textTransform: 'uppercase', letterSpacing: 1 }}>
            Explorer
          </div>
          <Tree
            treeData={treeData}
            selectedKeys={[selectedFile]}
            onSelect={(keys) => keys[0] && setSelectedFile(keys[0] as string)}
            defaultExpandAll
            style={{ background: 'transparent', color: '#cccccc' }}
            className="ide-tree"
          />
          <style>{`
            .ide-tree .ant-tree-node-content-wrapper { color: #cccccc !important; }
            .ide-tree .ant-tree-node-content-wrapper:hover { background: #2a2d2e !important; }
            .ide-tree .ant-tree-node-selected .ant-tree-node-content-wrapper,
            .ide-tree .ant-tree-node-content-wrapper.ant-tree-node-selected { background: #37373d !important; color: #ffffff !important; }
            .ide-tree .ant-tree-switcher { color: #cccccc !important; }
            .ide-tree .ant-tree-indent-unit { width: 16px; }
          `}</style>
        </div>

        {/* Monaco editor */}
        <div style={{ flex: 1, minWidth: 0 }}>
          <Editor
            height="100%"
            language={getLanguage(selectedFile)}
            value={currentFile?.content || '// No file selected'}
            theme="vs-dark"
            onMount={handleEditorMount}
            options={{
              readOnly: true,
              minimap: { enabled: true },
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
          height: 4, background: '#3c3c3c', cursor: 'ns-resize', flexShrink: 0,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}
      >
        <div style={{ width: 40, height: 2, background: '#666', borderRadius: 1 }} />
      </div>

      {/* Bottom panel: description + exercise controls */}
      <div style={{
        height: bottomPanelHeight, background: '#1e1e1e', borderTop: '1px solid #3c3c3c',
        overflow: 'auto', flexShrink: 0, padding: '12px 16px', color: '#cccccc',
      }}>
        {completed ? (
          <div>
            <Alert message="Exercise complete!" type="success" showIcon style={{ marginBottom: 8 }} />
            {typeof feedback?.explanation === 'string' && (
              <div style={{ background: '#252526', padding: 12, borderRadius: 4, color: '#cccccc' }}>
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
              <Typography.Text style={{ color: '#cccccc' }}>
                Click line numbers to select problematic lines.
                Selected: <strong>{selectedLines.length > 0 ? selectedLines.sort((a, b) => a - b).join(', ') : 'none'}</strong>
              </Typography.Text>
              <Button type="primary" size="small" onClick={handleSubmitIdentify} loading={submitting} disabled={selectedLines.length === 0}>
                Validate
              </Button>
            </div>
            {feedback && !feedback.is_correct && (
              <Alert
                message={`${feedback.matched}/${feedback.total} lines found`}
                description={feedback.hint as string}
                type="warning" showIcon style={{ marginTop: 8 }}
              />
            )}
            {feedback && (feedback.is_correct as boolean) && (
              <Alert message="Correct! Moving to fix phase..." type="success" showIcon style={{ marginTop: 8 }} />
            )}
          </div>
        ) : (
          <div>
            <div style={{ marginBottom: 8 }}>
              <Typography.Text strong style={{ color: '#cccccc' }}>Select the correct fix:</Typography.Text>
            </div>
            <Radio.Group
              onChange={(e) => setSelectedPatch(e.target.value)}
              value={selectedPatch}
              style={{ width: '100%' }}
              disabled={completed}
            >
              <Space direction="vertical" style={{ width: '100%' }}>
                {patches.map((p) => (
                  <Radio key={p.label} value={p.label} style={{ color: '#cccccc' }}>
                    <Tooltip title={p.diff} placement="topLeft" overlayStyle={{ maxWidth: 500 }} overlayInnerStyle={{ whiteSpace: 'pre', fontFamily: 'monospace', fontSize: 12 }}>
                      <Typography.Text style={{ color: '#cccccc' }}>{p.label}</Typography.Text>
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
