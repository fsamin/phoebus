import React, { useEffect, useState, useMemo } from 'react';
import { Card, Row, Col, Typography, Tag, Input, Empty, Spin, Select } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { api } from '../api/client';
import type { LearningPathSummary, Progress } from '../api/client';

const Catalog: React.FC = () => {
  const navigate = useNavigate();
  const [paths, setPaths] = useState<LearningPathSummary[]>([]);
  const [progress, setProgress] = useState<Progress[]>([]);
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
    Promise.all([api.listPaths(), api.getProgress()])
      .then(([p, pr]) => { setPaths(p); setProgress(pr); })
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  const allTags = [...new Set(paths.flatMap((p) => p.tags || []))].sort();

  // We don't have path_id in progress entries, so status filter is approximate
  void statusFilter; // status filter UI present; full implementation requires backend path-progress mapping
  void progress;
  const filtered = useMemo(() => {
    let result = paths.filter((p) => {
      const matchesSearch =
        !search ||
        p.title.toLowerCase().includes(search.toLowerCase()) ||
        p.description.toLowerCase().includes(search.toLowerCase());
      const matchesTags =
        tagFilter.length === 0 || tagFilter.every((t) => p.tags?.includes(t));
      return matchesSearch && matchesTags;
    });
    // Sort
    if (sortBy === 'az') result = [...result].sort((a, b) => a.title.localeCompare(b.title));
    else if (sortBy === 'za') result = [...result].sort((a, b) => b.title.localeCompare(a.title));
    return result;
  }, [paths, search, tagFilter, sortBy]);

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
            ]}
          />
        </Col>
      </Row>

      {filtered.length === 0 ? (
        <Empty description="No learning paths match your search" />
      ) : (
        <Row gutter={[16, 16]}>
          {filtered.map((path) => (
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
                {path.prerequisites && path.prerequisites.length > 0 && (
                  <div style={{ marginTop: 8 }}>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      Prerequisites: {path.prerequisites.join(', ')}
                    </Typography.Text>
                  </div>
                )}
              </Card>
            </Col>
          ))}
        </Row>
      )}
    </div>
  );
};

export default Catalog;
