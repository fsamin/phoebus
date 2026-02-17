import React, { useEffect, useState, useMemo } from 'react';
import { Card, Row, Col, Typography, Tag, Input, Empty, Spin, Select, Progress as AntProgress } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { api } from '../api/client';

interface CatalogPath {
  id: string;
  title: string;
  description: string;
  icon?: string;
  tags: string[];
  estimated_duration?: string;
  prerequisites?: string[];
  module_count: number;
  step_count: number;
  progress_total?: number;
  progress_completed?: number;
}

const Catalog: React.FC = () => {
  const navigate = useNavigate();
  const [paths, setPaths] = useState<CatalogPath[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');
  const [tagFilter, setTagFilter] = useState<string[]>([]);
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [sortBy, setSortBy] = useState<string>('az');

  // Debounce search 300ms
  useEffect(() => {
    const t = setTimeout(() => setSearch(searchInput), 300);
    return () => clearTimeout(t);
  }, [searchInput]);

  useEffect(() => {
    api.listPaths()
      .then((p) => setPaths(p as unknown as CatalogPath[]))
      .finally(() => setLoading(false));
  }, []);

  const allTags = [...new Set(paths.flatMap((p) => p.tags || []))].sort();

  const getPathStatus = (p: CatalogPath): string => {
    if (!p.progress_total) return 'not_started';
    if (p.progress_completed === p.progress_total) return 'completed';
    return 'in_progress';
  };

  const filtered = useMemo(() => {
    let result = paths.filter((p) => {
      const matchesSearch =
        !search ||
        p.title.toLowerCase().includes(search.toLowerCase()) ||
        p.description.toLowerCase().includes(search.toLowerCase());
      const matchesTags =
        tagFilter.length === 0 || tagFilter.every((t) => p.tags?.includes(t));
      const matchesStatus =
        statusFilter === 'all' || getPathStatus(p) === statusFilter;
      return matchesSearch && matchesTags && matchesStatus;
    });
    if (sortBy === 'az') result = [...result].sort((a, b) => a.title.localeCompare(b.title));
    else if (sortBy === 'za') result = [...result].sort((a, b) => b.title.localeCompare(a.title));
    else if (sortBy === 'progress') result = [...result].sort((a, b) => {
      const pa = a.progress_total ? (a.progress_completed ?? 0) / a.progress_total : -1;
      const pb = b.progress_total ? (b.progress_completed ?? 0) / b.progress_total : -1;
      return pb - pa;
    });
    return result;
  }, [paths, search, tagFilter, sortBy, statusFilter]);

  if (loading) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  return (
    <div>
      <Typography.Title level={2}>Catalog</Typography.Title>
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={6}>
          <Input
            prefix={<SearchOutlined />}
            placeholder="Search learning paths..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            allowClear
          />
        </Col>
        <Col xs={24} sm={6}>
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
        <Col xs={24} sm={6}>
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
        <Col xs={24} sm={6}>
          <Select
            style={{ width: '100%' }}
            value={sortBy}
            onChange={setSortBy}
            options={[
              { label: 'A → Z', value: 'az' },
              { label: 'Z → A', value: 'za' },
              { label: 'Progress ↓', value: 'progress' },
            ]}
          />
        </Col>
      </Row>

      {filtered.length === 0 ? (
        <Empty description="No learning paths match your search" />
      ) : (
        <Row gutter={[16, 16]}>
          {filtered.map((path) => {
            const pct = path.progress_total
              ? Math.round(((path.progress_completed ?? 0) / path.progress_total) * 100)
              : 0;
            const status = getPathStatus(path);
            return (
              <Col xs={24} sm={12} lg={8} key={path.id}>
                <Card
                  hoverable
                  onClick={() => navigate(`/paths/${path.id}`)}
                  style={{ height: '100%' }}
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
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        Prerequisites: {path.prerequisites.join(', ')}
                      </Typography.Text>
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
