import React, { useEffect, useState, useMemo } from 'react';
import { Card, Row, Col, Typography, Tag, Input, Empty, Spin, Select, Progress as AntProgress, Segmented } from 'antd';
import { SearchOutlined, CheckCircleOutlined, WarningOutlined, AppstoreOutlined, ApartmentOutlined } from '@ant-design/icons';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { api } from '../api/client';
import type { Competency, DependencyEdge } from '../api/client';
import { usePageTitle } from '../hooks/usePageTitle';
import OnboardingTour from '../components/OnboardingTour';
import { catalogSteps } from '../tours/steps';
import CatalogDAG from '../components/CatalogDAG';

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

const Catalog: React.FC = () => {
  usePageTitle('Catalog');
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [paths, setPaths] = useState<CatalogPath[]>([]);
  const [competencies, setCompetencies] = useState<Competency[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');
  const [tagFilter, setTagFilter] = useState<string[]>([]);
  const [competencyFilter, setCompetencyFilter] = useState<string[]>(
    searchParams.get('competencies')?.split(',').filter(Boolean) || []
  );
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [sortBy, setSortBy] = useState<string>('competency');
  const [viewMode, setViewMode] = useState<string>(() => localStorage.getItem('catalog-view') || 'grid');
  const [depEdges, setDepEdges] = useState<DependencyEdge[]>([]);

  // Debounce search 300ms
  useEffect(() => {
    const t = setTimeout(() => setSearch(searchInput), 300);
    return () => clearTimeout(t);
  }, [searchInput]);

  useEffect(() => {
    Promise.all([api.listPaths(), api.listCompetencies(), api.listPathDependencies()])
      .then(([p, c, d]) => {
        setPaths(p as unknown as CatalogPath[]);
        setCompetencies(c);
        setDepEdges(d.edges || []);
      })
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    localStorage.setItem('catalog-view', viewMode);
  }, [viewMode]);

  const allTags = [...new Set(paths.flatMap((p) => p.tags || []))].sort();
  const allCompetencies = [...new Set(competencies.map((c) => c.name))].sort();

  const getPathStatus = (p: CatalogPath): string => {
    if (!p.progress_total) return 'not_started';
    if (p.progress_completed === p.progress_total) return 'completed';
    return 'in_progress';
  };

  // Topological sort by competency dependencies
  const topoSort = (items: CatalogPath[]): CatalogPath[] => {
    // Build a map: competency -> paths that provide it
    const providedBy = new Map<string, Set<string>>();
    for (const p of items) {
      for (const c of p.competencies_provided || []) {
        if (!providedBy.has(c)) providedBy.set(c, new Set());
        providedBy.get(c)!.add(p.id);
      }
    }

    const sorted: CatalogPath[] = [];
    const visited = new Set<string>();
    const resolvedCompetencies = new Set<string>();

    // Iteratively pick paths whose prerequisites are all resolved
    let remaining = [...items];
    while (remaining.length > 0) {
      const batch = remaining.filter((p) => {
        if (!p.prerequisites || p.prerequisites.length === 0) return true;
        return p.prerequisites.every((pr) => resolvedCompetencies.has(pr));
      });

      if (batch.length === 0) {
        // Cycle detected — add remaining paths alphabetically
        remaining.sort((a, b) => a.title.localeCompare(b.title));
        sorted.push(...remaining);
        break;
      }

      // Sort batch alphabetically for determinism
      batch.sort((a, b) => a.title.localeCompare(b.title));
      for (const p of batch) {
        sorted.push(p);
        visited.add(p.id);
        for (const c of p.competencies_provided || []) {
          resolvedCompetencies.add(c);
        }
      }
      remaining = remaining.filter((p) => !visited.has(p.id));
    }
    return sorted;
  };

  const filtered = useMemo(() => {
    let result = paths.filter((p) => {
      const matchesSearch =
        !search ||
        p.title.toLowerCase().includes(search.toLowerCase()) ||
        p.description.toLowerCase().includes(search.toLowerCase());
      const matchesTags =
        tagFilter.length === 0 || tagFilter.every((t) => p.tags?.includes(t));
      const matchesCompetency =
        competencyFilter.length === 0 ||
        competencyFilter.some((c) => p.competencies_provided?.includes(c));
      const matchesStatus =
        statusFilter === 'all' || getPathStatus(p) === statusFilter;
      return matchesSearch && matchesTags && matchesCompetency && matchesStatus;
    });
    if (sortBy === 'az') result = [...result].sort((a, b) => a.title.localeCompare(b.title));
    else if (sortBy === 'za') result = [...result].sort((a, b) => b.title.localeCompare(a.title));
    else if (sortBy === 'progress') result = [...result].sort((a, b) => {
      const pa = a.progress_total ? (a.progress_completed ?? 0) / a.progress_total : -1;
      const pb = b.progress_total ? (b.progress_completed ?? 0) / b.progress_total : -1;
      return pb - pa;
    });
    else if (sortBy === 'competency') result = topoSort(result);
    return result;
  }, [paths, search, tagFilter, competencyFilter, sortBy, statusFilter]);

  if (loading) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  return (
    <div>
      <OnboardingTour tour="catalog" steps={catalogSteps} />
      <Typography.Title level={2} data-tour="catalog-title">Catalog</Typography.Title>
      <Row gutter={16} style={{ marginBottom: 24 }} data-tour="catalog-filters">
        <Col xs={24} sm={5}>
          <Input
            prefix={<SearchOutlined />}
            placeholder="Search learning paths..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            allowClear
            data-tour="catalog-search"
          />
        </Col>
        <Col xs={24} sm={5}>
          <Select
            mode="multiple"
            placeholder="Filter by tags"
            value={tagFilter}
            onChange={setTagFilter}
            options={allTags.map((t) => ({ label: t, value: t }))}
            style={{ width: '100%' }}
            allowClear
          />
        </Col>
        <Col xs={24} sm={5}>
          <Select
            mode="multiple"
            placeholder="Filter by competency"
            value={competencyFilter}
            onChange={setCompetencyFilter}
            options={allCompetencies.map((c) => ({ label: c, value: c }))}
            style={{ width: '100%' }}
            allowClear
          />
        </Col>
        <Col xs={24} sm={4}>
          <Select
            style={{ width: '100%' }}
            value={statusFilter}
            onChange={setStatusFilter}
            options={[
              { label: 'All Status', value: 'all' },
              { label: 'Not Started', value: 'not_started' },
              { label: 'In Progress', value: 'in_progress' },
              { label: 'Completed', value: 'completed' },
            ]}
          />
        </Col>
        <Col xs={24} sm={5}>
          <Select
            style={{ width: '100%' }}
            value={sortBy}
            onChange={setSortBy}
            options={[
              { label: 'A → Z', value: 'az' },
              { label: 'Z → A', value: 'za' },
              { label: 'Progress ↓', value: 'progress' },
              { label: 'Competency Path', value: 'competency' },
            ]}
          />
        </Col>
        <Col xs={24} sm={4} style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end' }}>
          <Segmented
            value={viewMode}
            onChange={(v) => setViewMode(v as string)}
            options={[
              { value: 'grid', icon: <AppstoreOutlined /> },
              { value: 'dag', icon: <ApartmentOutlined /> },
            ]}
          />
        </Col>
      </Row>

      {viewMode === 'dag' ? (
        <CatalogDAG paths={filtered as unknown as any[]} edges={depEdges} />
      ) : filtered.length === 0 ? (
        <Empty description="No learning paths match your search" />
      ) : (
        <Row gutter={[16, 16]}>
          {filtered.map((path, index) => {
            const pct = path.progress_total
              ? Math.round(((path.progress_completed ?? 0) / path.progress_total) * 100)
              : 0;
            const status = getPathStatus(path);
            return (
              <Col xs={24} sm={12} lg={8} key={path.id} {...(index === 0 ? { 'data-tour': 'catalog-cards' } : {})}>
                <Card
                  hoverable
                  onClick={() => navigate(`/paths/${path.slug}`)}                  style={{ height: '100%' }}
                >
                  <Card.Meta
                    title={
                      <span>
                        {path.icon && <span style={{ marginRight: 8 }}>{path.icon}</span>}
                        {path.title}
                      </span>
                    }
                    description={path.description}
                  />
                  <div style={{ marginTop: 12 }}>
                    {path.tags?.map((tag) => <Tag key={tag}>{tag}</Tag>)}
                  </div>
                  {path.competencies_provided?.length > 0 && (
                    <div style={{ marginTop: 8 }}>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>Provides: </Typography.Text>
                      {path.competencies_provided.map((c) => (
                        <Tag key={c} color="geekblue" style={{ fontSize: 11 }}>{c}</Tag>
                      ))}
                    </div>
                  )}
                  {path.owners?.length > 0 && (
                    <div style={{ marginTop: 8 }}>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        Instructor{path.owners.length > 1 ? 's' : ''}: {path.owners.join(', ')}
                      </Typography.Text>
                    </div>
                  )}
                  <div style={{ marginTop: 12, display: 'flex', justifyContent: 'space-between' }}>
                    <Typography.Text type="secondary">
                      {path.module_count} modules · {path.step_count} steps
                    </Typography.Text>
                    {path.estimated_duration && (
                      <Typography.Text type="secondary">⏱ {path.estimated_duration}</Typography.Text>
                    )}
                  </div>
                  {status !== 'not_started' ? (
                    <AntProgress percent={pct} size="small" style={{ marginTop: 8 }} />
                  ) : (
                    <Typography.Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
                      Not started
                    </Typography.Text>
                  )}
                  {path.prerequisites && path.prerequisites.length > 0 && (
                    <div style={{ marginTop: 8 }}>
                      {path.prerequisites_met ? (
                        <Tag icon={<CheckCircleOutlined />} color="success">Prerequisites met</Tag>
                      ) : (
                        <Tag icon={<WarningOutlined />} color="warning">Prerequisites not met</Tag>
                      )}
                    </div>
                  )}
                </Card>
              </Col>
            );
          })}
        </Row>
      )}
    </div>
  );
};

export default Catalog;
