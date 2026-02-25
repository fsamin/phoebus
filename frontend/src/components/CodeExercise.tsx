import React, { useState, useMemo, useRef, useEffect } from 'react';
import { Button, Alert, Typography, Tag, Tree } from 'antd';
import { FileOutlined, FolderOutlined, CheckCircleFilled, BugFilled, CloseCircleFilled, DiffOutlined, ArrowLeftOutlined, ArrowRightOutlined, ReloadOutlined } from '@ant-design/icons';
import Editor, { DiffEditor } from '@monaco-editor/react';
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
  onNext?: () => void;
  onBack?: () => void;
  onOverview?: () => void;
  nextLabel?: string;
  prevLabel?: string;
  onReset?: () => void;
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

// Parse a unified diff and apply it to original file contents
// Returns a map of filePath -> { original, modified } for each file in the diff
function applyUnifiedDiff(diff: string, files: CodebaseFile[]): Map<string, { original: string; modified: string }> {
  const result = new Map<string, { original: string; modified: string }>();
  if (!diff) return result;

  // Split into per-file sections
  const fileSections = diff.split(/^(?=--- )/m).filter(Boolean);

  for (const section of fileSections) {
    const lines = section.split('\n');
    // Extract file path from --- a/path or --- path
    const minusMatch = lines[0]?.match(/^--- (?:a\/)?(.+)/);
    const plusMatch = lines[1]?.match(/^\+\+\+ (?:b\/)?(.+)/);
    if (!minusMatch || !plusMatch) continue;

    const filePath = plusMatch[1];
    const sourceFile = files.find((f) => f.file_path === filePath || f.file_path.endsWith(filePath));
    if (!sourceFile) continue;

    const originalLines = sourceFile.content.split('\n');
    const modifiedLines = [...originalLines];
    let offset = 0; // track line shifts from insertions/deletions

    // Process each hunk
    const hunkRegex = /^@@ -(\d+)(?:,\d+)? \+\d+(?:,\d+)? @@/;
    for (let i = 2; i < lines.length; i++) {
      const hunkMatch = lines[i].match(hunkRegex);
      if (!hunkMatch) continue;

      let origLine = parseInt(hunkMatch[1], 10) - 1; // 0-based index in original
      let pos = origLine + offset; // position in modifiedLines
      i++;

      while (i < lines.length && !lines[i].match(hunkRegex) && !lines[i].startsWith('--- ')) {
        const line = lines[i];
        if (line.startsWith('-')) {
          modifiedLines.splice(pos, 1);
          offset--;
          // don't increment pos — next line is now at same pos
        } else if (line.startsWith('+')) {
          modifiedLines.splice(pos, 0, line.substring(1));
          offset++;
          pos++;
        } else if (line.startsWith(' ') || line === '') {
          pos++;
        }
        i++;
      }
      i--; // back up so the outer loop re-evaluates this line
    }

    result.set(sourceFile.file_path, {
      original: sourceFile.content,
      modified: modifiedLines.join('\n'),
    });
  }
  return result;
}

// Get file paths affected by a diff
function getAffectedFiles(diff: string): string[] {
  const paths: string[] = [];
  const matches = diff.matchAll(/^\+\+\+ (?:b\/)?(.+)$/gm);
  for (const m of matches) paths.push(m[1]);
  return paths;
}

const CodeExercise: React.FC<CodeExerciseProps> = ({ mode, description, target, patches, codebaseFiles, onSubmit, onNext, onBack, onOverview, nextLabel, prevLabel, onReset }) => {
  const { isDark } = useTheme();
  const [selectedFile, setSelectedFile] = useState(target?.file || codebaseFiles[0]?.file_path || '');
  const [selectedLines, setSelectedLines] = useState<number[]>([]);
  const [phase, setPhase] = useState<'identify' | 'fix'>(mode === 'B' ? 'fix' : 'identify');
  const [selectedPatch, setSelectedPatch] = useState('');
  const [feedback, setFeedback] = useState<Record<string, unknown> | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [completed, setCompleted] = useState(false);
  const [bottomPanelHeight, setBottomPanelHeight] = useState(200);
  const [disabledPatches, setDisabledPatches] = useState<Set<string>>(new Set());
  const editorRef = useRef<any>(null);
  const decorationsRef = useRef<string[]>([]);
  const monacoRef = useRef<any>(null);
  const resizingRef = useRef(false);

  const currentFile = codebaseFiles.find((f) => f.file_path === selectedFile);
  const treeData = useMemo(() => buildTreeData(codebaseFiles), [codebaseFiles]);

  // Compute diff data for selected patch
  const selectedPatchObj = patches.find((p) => p.label === selectedPatch);
  const diffData = useMemo(() => {
    if (!selectedPatchObj?.diff) return null;
    return applyUnifiedDiff(selectedPatchObj.diff, codebaseFiles);
  }, [selectedPatchObj?.diff, codebaseFiles]); // eslint-disable-line react-hooks/exhaustive-deps
  const affectedFiles = useMemo(() => selectedPatchObj?.diff ? getAffectedFiles(selectedPatchObj.diff) : [], [selectedPatchObj?.diff]);
  const showDiffEditor = phase === 'fix' && selectedPatch && diffData && diffData.has(selectedFile);

  // Auto-navigate to first affected file when a patch is selected
  useEffect(() => {
    if (affectedFiles.length > 0 && selectedPatch) {
      const match = codebaseFiles.find((f) => affectedFiles.some((af) => f.file_path === af || f.file_path.endsWith(af)));
      if (match) setSelectedFile(match.file_path);
    }
  }, [selectedPatch]); // eslint-disable-line react-hooks/exhaustive-deps

  // Update editor decorations when selected lines change
  useEffect(() => {
    const editor = editorRef.current;
    const monaco = monacoRef.current;
    if (!editor || !monaco) return;
    const newDecorations: any[] = selectedLines.map((lineNum) => ({
      range: new monaco.Range(lineNum, 1, lineNum, 1),
      options: {
        isWholeLine: true,
        className: 'line-selected',
        glyphMarginClassName: 'line-glyph-selected',
      },
    }));
    if (phase === 'fix' && target?.file === selectedFile) {
      target.lines.forEach((lineNum: number) => {
        newDecorations.push({
          range: new monaco.Range(lineNum, 1, lineNum, 1),
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

  const handleEditorMount = (editor: any, monaco: any) => {
    editorRef.current = editor;
    monacoRef.current = monaco;
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
  };

  const handleSubmitIdentify = async () => {
    setSubmitting(true);
    try {
      const result = await onSubmit({ phase: 'identify', selected_lines: selectedLines });
      setFeedback(result);
      // Auto-expand panel to show feedback
      setBottomPanelHeight((h) => Math.max(h, 220));
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
      setBottomPanelHeight((h) => Math.max(h, 220));
      if (result.is_correct) {
        setCompleted(true);
      } else {
        setDisabledPatches((prev) => new Set([...prev, selectedPatch]));
        setSelectedPatch('');
      }
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

      {/* Top bar */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '0 12px', height: 36, background: 'var(--color-bg-ide-secondary)', borderBottom: '1px solid var(--color-border-ide)',
        flexShrink: 0,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {showDiffEditor ? <DiffOutlined style={{ color: 'var(--color-primary)', fontSize: 13 }} /> : <FileOutlined style={{ color: 'var(--color-text-ide)', fontSize: 13 }} />}
          <Typography.Text style={{ color: 'var(--color-text-ide)', fontSize: 13 }}>{selectedFile}</Typography.Text>
          {showDiffEditor && <Tag color="var(--color-primary)" style={{ fontSize: 11, lineHeight: '18px', padding: '0 6px' }}>DIFF</Tag>}
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

        {/* Monaco editor / Diff editor */}
        <div style={{ flex: 1, minWidth: 0 }}>
          {showDiffEditor ? (
            <DiffEditor
              height="100%"
              language={getLanguage(selectedFile)}
              original={diffData.get(selectedFile)!.original}
              modified={diffData.get(selectedFile)!.modified}
              theme={isDark ? 'vs-dark' : 'vs'}
              options={{
                readOnly: true,
                renderSideBySide: true,
                minimap: { enabled: false },
                fontSize: 14,
                lineNumbers: 'on',
                scrollBeyondLastLine: false,
                automaticLayout: true,
                contextmenu: false,
              }}
            />
          ) : (
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
          )}
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
        transition: 'height 0.2s ease',
      }}>
        {completed ? (
          <div>
            <Alert message="Exercise complete!" type="success" showIcon style={{ marginBottom: 12 }} />
            {typeof feedback?.explanation === 'string' && (
              <div style={{ background: 'var(--color-bg-ide-secondary)', padding: 12, borderRadius: 4, marginBottom: 12 }}>
                <MarkdownRenderer content={feedback.explanation} />
              </div>
            )}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', paddingTop: 8, borderTop: '1px solid var(--color-border-ide)' }}>
              <div style={{ display: 'flex', gap: 8 }}>
                {onBack && (
                  <Button icon={<ArrowLeftOutlined />} onClick={onBack}>
                    {prevLabel || 'Previous'}
                  </Button>
                )}
                {onReset && (
                  <Button icon={<ReloadOutlined />} onClick={onReset}>Reset</Button>
                )}
              </div>
              {onNext ? (
                <Button type="primary" onClick={onNext}>
                  {nextLabel || 'Next'} <ArrowRightOutlined />
                </Button>
              ) : onOverview ? (
                <Button type="primary" onClick={onOverview}>
                  Back to Overview <ArrowRightOutlined />
                </Button>
              ) : null}
            </div>
          </div>
        ) : phase === 'identify' ? (
          <div>
            {/* Show feedback OR description, not both */}
            {feedback && !feedback.is_correct ? (
              <Alert
                message={`${feedback.matched}/${feedback.total} lines found`}
                description={feedback.hint as string}
                type="warning"
                showIcon
                style={{ marginBottom: 8 }}
              />
            ) : feedback?.is_correct ? (
              <Alert message="Correct! Moving to fix phase..." type="success" showIcon style={{ marginBottom: 8 }} />
            ) : (
              <div style={{ marginBottom: 8 }}>
                <MarkdownRenderer content={description} />
              </div>
            )}
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <Typography.Text style={{ color: 'var(--color-text-ide)' }}>
                Click line numbers to select problematic lines.
                Selected: <strong>{selectedLines.length > 0 ? selectedLines.sort((a, b) => a - b).join(', ') : 'none'}</strong>
              </Typography.Text>
              <Button type="primary" size="small" onClick={handleSubmitIdentify} loading={submitting} disabled={selectedLines.length === 0}>
                Validate
              </Button>
            </div>
          </div>
        ) : (
          <div>
            {/* Show feedback OR patch selection */}
            {feedback ? (
              <Alert
                message={feedback.is_correct ? 'Correct!' : 'Incorrect'}
                description={feedback.explanation as string}
                type={feedback.is_correct ? 'success' : 'error'}
                showIcon
                style={{ marginBottom: 8 }}
              />
            ) : (
              <>
                <div style={{ marginBottom: 8 }}>
                  <Typography.Text strong style={{ color: 'var(--color-text-ide)', fontSize: 13 }}>
                    Select the correct fix:
                  </Typography.Text>
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                  {patches.map((p) => {
                    const isDisabled = disabledPatches.has(p.label);
                    const isSelected = selectedPatch === p.label;
                    return (
                      <div
                        key={p.label}
                        onClick={() => { if (!isDisabled) setSelectedPatch(p.label); }}
                        style={{
                          padding: '8px 12px',
                          borderRadius: 6,
                          cursor: isDisabled ? 'not-allowed' : 'pointer',
                          border: `1px solid ${isSelected ? 'var(--color-primary)' : 'var(--color-border-ide)'}`,
                          background: isSelected ? 'var(--color-ide-patch-selected)' : 'var(--color-bg-ide-secondary)',
                          opacity: isDisabled ? 0.4 : 1,
                          transition: 'all 0.15s',
                          display: 'flex',
                          alignItems: 'flex-start',
                          gap: 10,
                        }}
                      >
                        <Typography.Text strong style={{ color: isSelected ? 'var(--color-primary)' : 'var(--color-text-ide)', fontSize: 13, whiteSpace: 'nowrap' }}>
                          {p.label}
                        </Typography.Text>
                        <pre style={{
                          margin: 0, flex: 1, fontSize: 12, lineHeight: 1.5, overflow: 'hidden',
                          fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
                          color: 'var(--color-text-ide-secondary)',
                          whiteSpace: 'pre-wrap', wordBreak: 'break-all',
                          maxHeight: 60,
                        }}>
                          {p.diff}
                        </pre>
                        {isDisabled && <CloseCircleFilled style={{ color: 'var(--color-danger)', marginTop: 2 }} />}
                      </div>
                    );
                  })}
                </div>
                <div style={{ marginTop: 10 }}>
                  <Button type="primary" size="small" onClick={handleSubmitFix} loading={submitting} disabled={!selectedPatch}>
                    Submit Fix
                  </Button>
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
};

export default CodeExercise;
