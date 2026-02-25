import React, { useEffect, useRef } from 'react';
import ReactMarkdown from 'react-markdown';
import type { Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkDirective from 'remark-directive';
import remarkDirectiveRehype from 'remark-directive-rehype';
import rehypeHighlight from 'rehype-highlight';
import rehypeRaw from 'rehype-raw';
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize';
import DOMPurify from 'dompurify';
import mermaid from 'mermaid';
import 'highlight.js/styles/github.css';

mermaid.initialize({ startOnLoad: false, theme: 'default', securityLevel: 'strict' });

// Sanitization schema: extend default to allow hljs classes and admonition directives
// Block dangerous protocols (file://, javascript:, data: in links)
const sanitizeSchema = {
  ...defaultSchema,
  attributes: {
    ...defaultSchema.attributes,
    // Allow className on all elements (needed for hljs, admonitions, mermaid code blocks)
    '*': [...(defaultSchema.attributes?.['*'] || []), 'className'],
    code: [...(defaultSchema.attributes?.['code'] || []), 'className'],
    span: [...(defaultSchema.attributes?.['span'] || []), 'className'],
    div: [...(defaultSchema.attributes?.['div'] || []), 'className'],
  },
  // Block dangerous tags (script, style, iframe, object, embed, form)
  tagNames: (defaultSchema.tagNames || []).filter(
    (tag: string) => !['script', 'style', 'iframe', 'object', 'embed', 'form', 'textarea'].includes(tag)
  ),
  // Only allow safe URL protocols
  protocols: {
    ...defaultSchema.protocols,
    href: ['http', 'https', 'mailto'],
    src: ['http', 'https'],
    cite: ['http', 'https'],
  },
};

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
        const sanitizedSvg = DOMPurify.sanitize(svg, { USE_PROFILES: { svg: true, svgFilters: true }, ADD_TAGS: ['foreignObject'] });
        const wrapper = document.createElement('div');
        wrapper.className = 'mermaid-diagram';
        wrapper.innerHTML = sanitizedSvg;
        parent.replaceWith(wrapper);
      } catch {
        // leave code block as-is if rendering fails
      }
    });
  }, [content]);

  return (
    <div className="markdown-body" ref={containerRef}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkDirective, remarkDirectiveRehype]}
        rehypePlugins={[rehypeHighlight, rehypeRaw, [rehypeSanitize, sanitizeSchema]]}
        components={components}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
};

export default MarkdownRenderer;
