import React, { useMemo, useCallback } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  Handle,
  type Node,
  type Edge,
  Position,
  useNodesState,
  useEdgesState,
  MarkerType,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import dagre from 'dagre';
import { Typography, Tag, Progress as AntProgress, Popover, Button } from 'antd';
import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  WarningOutlined,
  RightOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { useTheme } from '../contexts/ThemeContext';
import type { DependencyEdge } from '../api/client';

interface CatalogPath {
  id: string;
  slug: string;
  title: string;
  description: string;
  icon?: string;
  tags: string[];
  estimated_duration?: string;
  prerequisites?: string[];
  competencies_provided: string[];
  prerequisites_met: boolean;
  module_count: number;
  step_count: number;
  progress_total?: number;
  progress_completed?: number;
  owners: string[];
}

interface CatalogDAGProps {
  paths: CatalogPath[];
  edges: DependencyEdge[];
}

const NODE_WIDTH = 300;
const NODE_HEIGHT = 140;

function getProgressStatus(path: CatalogPath): 'completed' | 'in_progress' | 'not_started' {
  if (!path.progress_total || path.progress_total === 0) return 'not_started';
  if (path.progress_completed === path.progress_total) return 'completed';
  if ((path.progress_completed ?? 0) > 0) return 'in_progress';
  return 'not_started';
}

function getProgressPercent(path: CatalogPath): number {
  if (!path.progress_total || path.progress_total === 0) return 0;
  return Math.round(((path.progress_completed ?? 0) / path.progress_total) * 100);
}

const borderColors = {
  completed: '#52c41a',
  in_progress: '#fa8c16',
  not_started: '#d9d9d9',
};

function getLayout(nodes: Node[], edges: Edge[]): { treeNodes: Node[]; isolatedNodes: Node[] } {
  // Identify nodes that participate in edges vs isolated nodes
  const connectedIds = new Set<string>();
  edges.forEach((edge) => {
    connectedIds.add(edge.source);
    connectedIds.add(edge.target);
  });

  const connectedNodes = nodes.filter((n) => connectedIds.has(n.id));
  const isolatedNodes = nodes.filter((n) => !connectedIds.has(n.id));

  // Sort connected nodes alphabetically for deterministic layout
  const sortedConnected = [...connectedNodes].sort((a, b) =>
    (a.data.path as CatalogPath).title.localeCompare((b.data.path as CatalogPath).title)
  );

  // Layout connected nodes with dagre (top-to-bottom)
  let layoutConnected: Node[] = [];
  let maxY = 0;
  if (sortedConnected.length > 0) {
    const g = new dagre.graphlib.Graph();
    g.setDefaultEdgeLabel(() => ({}));
    g.setGraph({ rankdir: 'TB', nodesep: 80, ranksep: 160 });

    sortedConnected.forEach((node) => {
      g.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
    });
    edges.forEach((edge) => {
      g.setEdge(edge.source, edge.target);
    });

    dagre.layout(g);

    layoutConnected = sortedConnected.map((node) => {
      const pos = g.node(node.id);
      const y = pos.y - NODE_HEIGHT / 2;
      if (y + NODE_HEIGHT > maxY) maxY = y + NODE_HEIGHT;
      return {
        ...node,
        position: { x: pos.x - NODE_WIDTH / 2, y },
      };
    });
  }

  // Place isolated nodes in a horizontal row below the tree
  const ISOLATED_GAP = 40;
  const ISOLATED_TOP_MARGIN = 80;
  const sortedIsolated = [...isolatedNodes].sort((a, b) =>
    (a.data.path as CatalogPath).title.localeCompare((b.data.path as CatalogPath).title)
  );
  const isolatedStartY = maxY + ISOLATED_TOP_MARGIN;
  const totalIsolatedWidth = sortedIsolated.length * NODE_WIDTH + (sortedIsolated.length - 1) * ISOLATED_GAP;
  // Center isolated row relative to tree width
  const treeMinX = layoutConnected.length > 0
    ? Math.min(...layoutConnected.map((n) => n.position.x))
    : 0;
  const treeMaxX = layoutConnected.length > 0
    ? Math.max(...layoutConnected.map((n) => n.position.x + NODE_WIDTH))
    : totalIsolatedWidth;
  const treeCenterX = (treeMinX + treeMaxX) / 2;
  const isolatedStartX = treeCenterX - totalIsolatedWidth / 2;

  const layoutIsolated = sortedIsolated.map((node, i) => ({
    ...node,
    position: {
      x: isolatedStartX + i * (NODE_WIDTH + ISOLATED_GAP),
      y: isolatedStartY,
    },
  }));

  return { treeNodes: layoutConnected, isolatedNodes: layoutIsolated };
}

// Custom node component
const PathNode: React.FC<{ data: { path: CatalogPath } }> = ({ data }) => {
  const { path } = data;
  const status = getProgressStatus(path);
  const percent = getProgressPercent(path);
  const navigate = useNavigate();
  const { isDark } = useTheme();

  const nodeContent = (
    <div
      style={{
        width: NODE_WIDTH,
        height: NODE_HEIGHT,
        padding: 12,
        background: isDark ? '#1f1f1f' : '#fff',
        borderRadius: 8,
        border: `2px solid ${borderColors[status]}`,
        cursor: 'pointer',
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'space-between',
        boxShadow: isDark ? '0 2px 8px rgba(0,0,0,0.3)' : '0 2px 8px rgba(0,0,0,0.08)',
      }}
    >
      <div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
          {path.icon && <span style={{ fontSize: 18 }}>{path.icon}</span>}
          <Typography.Text strong ellipsis style={{ flex: 1, fontSize: 14 }}>
            {path.title}
          </Typography.Text>
          {status === 'completed' && <CheckCircleOutlined style={{ color: '#52c41a' }} />}
          {status === 'in_progress' && <ClockCircleOutlined style={{ color: '#fa8c16' }} />}
        </div>
        <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
          {path.tags.slice(0, 3).map((tag) => (
            <Tag key={tag} style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>{tag}</Tag>
          ))}
        </div>
      </div>
      <div>
        {!path.prerequisites_met && (
          <div style={{ fontSize: 11, color: '#fa8c16', marginBottom: 2 }}>
            <WarningOutlined /> Prerequisites not met
          </div>
        )}
        <AntProgress percent={percent} size="small" showInfo={false} strokeColor={borderColors[status]} />
        <div style={{ fontSize: 11, color: isDark ? '#a0a0a0' : '#8c8c8c' }}>
          {path.module_count} modules · {path.step_count} steps
          {path.estimated_duration && ` · ${path.estimated_duration}`}
        </div>
      </div>
    </div>
  );

  const popoverContent = (
    <div style={{ maxWidth: 300 }}>
      <Typography.Paragraph ellipsis={{ rows: 3 }} style={{ marginBottom: 8 }}>
        {path.description}
      </Typography.Paragraph>
      {path.competencies_provided.length > 0 && (
        <div style={{ marginBottom: 8 }}>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>Competencies:</Typography.Text>
          <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap', marginTop: 4 }}>
            {path.competencies_provided.map((c) => (
              <Tag key={c} color="blue" style={{ fontSize: 11 }}>{c}</Tag>
            ))}
          </div>
        </div>
      )}
      {path.prerequisites && path.prerequisites.length > 0 && (
        <div style={{ marginBottom: 8 }}>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>Prerequisites:</Typography.Text>
          <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap', marginTop: 4 }}>
            {path.prerequisites.map((p) => (
              <Tag key={p} color="orange" style={{ fontSize: 11 }}>{p}</Tag>
            ))}
          </div>
        </div>
      )}
      <Button type="primary" size="small" icon={<RightOutlined />} onClick={() => navigate(`/paths/${path.slug}`)}>
        View Path
      </Button>
    </div>
  );

  return (
    <>
      <Handle type="target" position={Position.Top} style={{ background: isDark ? '#555' : '#d9d9d9' }} />
      <Popover content={popoverContent} title={`${path.icon || ''} ${path.title}`} trigger="click" placement="right">
        {nodeContent}
      </Popover>
      <Handle type="source" position={Position.Bottom} style={{ background: isDark ? '#555' : '#d9d9d9' }} />
    </>
  );
};

const nodeTypes = { pathNode: PathNode };

const CatalogDAG: React.FC<CatalogDAGProps> = ({ paths, edges: depEdges }) => {
  const pathMap = useMemo(() => new Map(paths.map((p) => [p.id, p])), [paths]);
  const pathIds = useMemo(() => new Set(paths.map((p) => p.id)), [paths]);
  const { isDark } = useTheme();

  // Build nodes and edges
  const { initialNodes, initialEdges } = useMemo(() => {
    const rfNodes: Node[] = paths.map((p) => ({
      id: p.id,
      type: 'pathNode',
      data: { path: p },
      position: { x: 0, y: 0 },
      sourcePosition: Position.Bottom,
      targetPosition: Position.Top,
    }));

    const rfEdges: Edge[] = depEdges
      .filter((e) => pathIds.has(e.source) && pathIds.has(e.target))
      .map((e, i) => ({
        id: `edge-${i}`,
        source: e.source,
        target: e.target,
        type: 'smoothstep',
        animated: e.type !== 'auto',
        style: { stroke: e.type === 'auto' ? '#8c8c8c' : '#b37feb', strokeWidth: 2, opacity: 0.6 },
        markerEnd: { type: MarkerType.ArrowClosed, color: e.type === 'auto' ? '#8c8c8c' : '#b37feb' },
        label: e.competencies?.join(', '),
        labelStyle: { fontSize: 10, fill: isDark ? '#b0b0b0' : '#8c8c8c', fontWeight: 500 },
        labelBgStyle: { fill: isDark ? '#1f1f1f' : '#fff', fillOpacity: 0.85 },
        labelBgPadding: [4, 2] as [number, number],
        pathOptions: { borderRadius: 12 },
      }));

    const { treeNodes, isolatedNodes } = getLayout(rfNodes, rfEdges);
    const layoutNodes = [...treeNodes, ...isolatedNodes];

    // Add a label node for isolated paths section if there are any
    if (isolatedNodes.length > 0 && treeNodes.length > 0) {
      const labelY = Math.min(...isolatedNodes.map((n) => n.position.y)) - 40;
      const labelX = isolatedNodes.reduce((sum, n) => sum + n.position.x, 0) / isolatedNodes.length + NODE_WIDTH / 2 - 80;
      layoutNodes.push({
        id: '__isolated-label__',
        type: 'default',
        data: { label: '📚 Independent paths' },
        position: { x: labelX, y: labelY },
        selectable: false,
        draggable: false,
        style: {
          background: 'transparent',
          border: 'none',
          fontSize: 13,
          color: isDark ? '#8c8c8c' : '#595959',
          fontStyle: 'italic',
          width: 'auto',
          padding: 0,
        },
      } as Node);
    }

    return { initialNodes: layoutNodes, initialEdges: rfEdges };
  }, [paths, depEdges, pathIds, isDark]);

  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes);
  const [edgesState, setEdges, onEdgesChange] = useEdgesState(initialEdges);

  // Update nodes and edges when data changes
  React.useEffect(() => {
    setNodes(initialNodes);
    setEdges(initialEdges);
  }, [initialNodes, initialEdges, setNodes, setEdges]);

  const onInit = useCallback((instance: any) => {
    setTimeout(() => instance.fitView({ padding: 0.2 }), 100);
  }, []);

  if (paths.length === 0) {
    return <div style={{ textAlign: 'center', padding: 40, color: '#8c8c8c' }}>No learning paths match your filters.</div>;
  }

  return (
    <div style={{ width: '100%', height: 'calc(100vh - 280px)', minHeight: 400, border: `1px solid ${isDark ? '#303030' : '#f0f0f0'}`, borderRadius: 8, background: isDark ? '#141414' : undefined }}>
      <ReactFlow
        nodes={nodes}
        edges={edgesState}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        onInit={onInit}
        fitView
        minZoom={0.3}
        maxZoom={1.5}
        proOptions={{ hideAttribution: true }}
        style={{ background: isDark ? '#141414' : undefined }}
      >
        <Background color={isDark ? '#303030' : undefined} />
        <Controls />
        <MiniMap
          nodeColor={(node) => {
            const path = pathMap.get(node.id);
            if (!path) return '#d9d9d9';
            return borderColors[getProgressStatus(path)];
          }}
          style={{ borderRadius: 4, background: isDark ? '#1f1f1f' : undefined }}
        />
      </ReactFlow>
    </div>
  );
};

export default CatalogDAG;
