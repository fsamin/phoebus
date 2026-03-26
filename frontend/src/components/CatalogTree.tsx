import React, { useMemo } from 'react';
import { Tree, Tag, Progress as AntProgress, Typography, Empty } from 'antd';
import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  WarningOutlined,
  BookOutlined,
  FolderOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { useTheme } from '../contexts/ThemeContext';
import type { DependencyEdge } from '../api/client';
import type { DataNode } from 'antd/es/tree';

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

interface CatalogTreeProps {
  paths: CatalogPath[];
  edges: DependencyEdge[];
}

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

const statusColors = {
  completed: '#52c41a',
  in_progress: '#fa8c16',
  not_started: '#d9d9d9',
};

const PathNodeTitle: React.FC<{ path: CatalogPath; isDark: boolean }> = ({ path, isDark }) => {
  const navigate = useNavigate();
  const status = getProgressStatus(path);
  const percent = getProgressPercent(path);

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 12,
        padding: '8px 12px',
        borderRadius: 6,
        border: `1px solid ${isDark ? '#303030' : '#f0f0f0'}`,
        background: isDark ? '#1f1f1f' : '#fff',
        cursor: 'pointer',
        minWidth: 300,
        maxWidth: 600,
      }}
      onClick={() => navigate(`/paths/${path.slug}`)}
    >
      <span style={{ fontSize: 20, flexShrink: 0 }}>{path.icon || '📘'}</span>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <Typography.Text strong style={{ fontSize: 14 }}>
            {path.title}
          </Typography.Text>
          {status === 'completed' && <CheckCircleOutlined style={{ color: '#52c41a' }} />}
          {status === 'in_progress' && <ClockCircleOutlined style={{ color: '#fa8c16' }} />}
          {!path.prerequisites_met && (
            <WarningOutlined style={{ color: '#fa8c16' }} title="Prerequisites not met" />
          )}
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 2 }}>
          <span style={{ fontSize: 12, color: isDark ? '#a0a0a0' : '#8c8c8c', whiteSpace: 'nowrap' }}>
            {path.module_count} modules · {path.step_count} steps
            {path.estimated_duration && ` · ${path.estimated_duration}`}
          </span>
          {path.tags.slice(0, 3).map((tag) => (
            <Tag key={tag} style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px', margin: 0 }}>{tag}</Tag>
          ))}
        </div>
        <AntProgress
          percent={percent}
          size="small"
          showInfo={false}
          strokeColor={statusColors[status]}
          style={{ marginTop: 4, marginBottom: 0 }}
        />
      </div>
    </div>
  );
};

const CatalogTree: React.FC<CatalogTreeProps> = ({ paths, edges: depEdges }) => {
  const { isDark } = useTheme();

  const treeData = useMemo(() => {
    const pathMap = new Map(paths.map((p) => [p.id, p]));
    const pathIds = new Set(paths.map((p) => p.id));

    // Filter edges to only include visible paths
    const validEdges = depEdges.filter((e) => pathIds.has(e.source) && pathIds.has(e.target));

    // Build parent→children map (source provides prereqs for target)
    const childrenOf = new Map<string, string[]>();
    const hasParent = new Set<string>();

    validEdges.forEach((e) => {
      const children = childrenOf.get(e.source) || [];
      children.push(e.target);
      childrenOf.set(e.source, children);
      hasParent.add(e.target);
    });

    // Connected nodes that participate in edges
    const connectedIds = new Set<string>();
    validEdges.forEach((e) => {
      connectedIds.add(e.source);
      connectedIds.add(e.target);
    });

    // Build tree nodes recursively (with cycle detection)
    const buildNode = (id: string, visited: Set<string>): DataNode | null => {
      const path = pathMap.get(id);
      if (!path || visited.has(id)) return null;

      visited.add(id);
      const childIds = childrenOf.get(id) || [];
      const children = childIds
        .map((cid) => buildNode(cid, new Set(visited)))
        .filter((n): n is DataNode => n !== null)
        .sort((a, b) => String(a.title).localeCompare(String(b.title)));

      return {
        key: id,
        title: <PathNodeTitle path={path} isDark={isDark} />,
        icon: null,
        children: children.length > 0 ? children : undefined,
      };
    };

    // Root nodes: connected nodes that are not targets (no parents)
    const rootConnected = [...connectedIds]
      .filter((id) => !hasParent.has(id))
      .map((id) => buildNode(id, new Set()))
      .filter((n): n is DataNode => n !== null)
      .sort((a, b) => String(a.key).localeCompare(String(b.key)));

    // Isolated nodes: paths not in any edge
    const isolatedNodes: DataNode[] = paths
      .filter((p) => !connectedIds.has(p.id))
      .sort((a, b) => a.title.localeCompare(b.title))
      .map((p) => ({
        key: p.id,
        title: <PathNodeTitle path={p} isDark={isDark} />,
        icon: null,
        isLeaf: true,
      }));

    const result: DataNode[] = [];

    if (rootConnected.length > 0) {
      result.push({
        key: '__learning-tracks__',
        title: (
          <Typography.Text strong style={{ fontSize: 14 }}>
            <FolderOutlined style={{ marginRight: 6 }} />
            Learning Tracks
          </Typography.Text>
        ),
        children: rootConnected,
        selectable: false,
      });
    }

    if (isolatedNodes.length > 0) {
      result.push({
        key: '__independent__',
        title: (
          <Typography.Text strong style={{ fontSize: 14 }}>
            <BookOutlined style={{ marginRight: 6 }} />
            Independent Paths
          </Typography.Text>
        ),
        children: isolatedNodes,
        selectable: false,
      });
    }

    return result;
  }, [paths, depEdges, isDark]);

  if (paths.length === 0) {
    return <Empty description="No learning paths match your filters." />;
  }

  return (
    <div
      style={{
        padding: 16,
        border: `1px solid ${isDark ? '#303030' : '#f0f0f0'}`,
        borderRadius: 8,
        background: isDark ? '#141414' : undefined,
        minHeight: 300,
      }}
    >
      <Tree
        treeData={treeData}
        defaultExpandAll
        showLine={{ showLeafIcon: false }}
        showIcon={false}
        selectable={false}
        blockNode
        style={{ background: 'transparent' }}
      />
    </div>
  );
};

export default CatalogTree;
