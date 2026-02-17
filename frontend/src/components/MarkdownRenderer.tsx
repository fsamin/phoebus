import React, { useEffect, useRef } from 'react';
import ReactMarkdown from 'react-markdown';
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

  // Transform :::tip, :::warning, :::danger, :::info, :::note directives
  const processedContent = content.replace(
    /^:::(tip|warning|danger|info|note)\s*\n([\s\S]*?)^:::/gm,
    (_match, type: string, body: string) => {
      const colors: Record<string, string> = {
        tip: '#52c41a', warning: '#faad14', danger: '#ff4d4f', info: '#1890ff', note: '#722ed1',
      };
      const icons: Record<string, string> = {
        tip: '💡', warning: '⚠️', danger: '🚨', info: 'ℹ️', note: '📝',
      };
      return `<div style="border-left: 4px solid ${colors[type] || '#1890ff'}; padding: 12px 16px; margin: 16px 0; background: ${colors[type] || '#1890ff'}10; border-radius: 4px;"><strong>${icons[type] || ''} ${type.charAt(0).toUpperCase() + type.slice(1)}</strong>\n\n${body.trim()}</div>`;
    }
  );

  return (
    <div className="markdown-body" ref={containerRef}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkDirective, remarkDirectiveRehype]}
        rehypePlugins={[rehypeHighlight, rehypeRaw]}
      >
        {processedContent}
      </ReactMarkdown>
    </div>
  );
};

export default MarkdownRenderer;
