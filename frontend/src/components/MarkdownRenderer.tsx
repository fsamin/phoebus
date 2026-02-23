import React, { useEffect, useRef } from 'react';
import ReactMarkdown from 'react-markdown';
import type { Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkDirective from 'remark-directive';
import remarkDirectiveRehype from 'remark-directive-rehype';
import rehypeHighlight from 'rehype-highlight';
import rehypeRaw from 'rehype-raw';
import mermaid from 'mermaid';
import 'highlight.js/styles/github.css';

mermaid.initialize({ startOnLoad: false, theme: 'default' });

interface MarkdownRendererProps {
  content: string;
}

const admonitionStyles: Record<string, { color: string; icon: string }> = {
  tip:     { color: 'var(--color-success)', icon: '💡' },
  warning: { color: 'var(--color-warning)', icon: '⚠️' },
  danger:  { color: 'var(--color-danger)', icon: '🚨' },
  info:    { color: 'var(--color-info)', icon: 'ℹ️' },
  note:    { color: 'var(--color-note)', icon: '📝' },
  caution: { color: 'var(--color-warning)', icon: '⚠️' },
};

function Admonition({ type, children }: { type: string; children?: React.ReactNode }) {
  const style = admonitionStyles[type] || admonitionStyles.info;
  return (
    <div style={{
      borderLeft: `4px solid ${style.color}`,
      padding: '12px 16px',
      margin: '16px 0',
      background: 'var(--color-bg-elevated)',
      borderRadius: 4,
    }}>
      <strong>{style.icon} {type.charAt(0).toUpperCase() + type.slice(1)}</strong>
      <div style={{ marginTop: 8 }}>{children}</div>
    </div>
  );
}

const admonitionNames = Object.keys(admonitionStyles);

const components: Components = Object.fromEntries(
  admonitionNames.map((name) => [
    name,
    ({ children, ...props }: React.HTMLAttributes<HTMLDivElement> & { children?: React.ReactNode }) => (
      <Admonition type={name} {...props}>{children}</Admonition>
    ),
  ])
);

const MarkdownRenderer: React.FC<MarkdownRendererProps> = ({ content }) => {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!containerRef.current) return;
    const mermaidBlocks = containerRef.current.querySelectorAll('code.language-mermaid');
    mermaidBlocks.forEach(async (block, i) => {
      const parent = block.parentElement;
      if (!parent) return;
      const id = `mermaid-${Date.now()}-${i}`;
      try {
        const { svg } = await mermaid.render(id, block.textContent || '');
        parent.outerHTML = `<div class="mermaid-diagram">${svg}</div>`;
      } catch {
        // leave code block as-is if rendering fails
      }
    });
  }, [content]);

  return (
    <div className="markdown-body" ref={containerRef}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkDirective, remarkDirectiveRehype]}
        rehypePlugins={[rehypeHighlight, rehypeRaw]}
        components={components}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
};

export default MarkdownRenderer;
