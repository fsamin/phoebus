import React, { useEffect, useState } from 'react';
import { Card, Row, Col, Typography, Tag, Input, Empty, Spin, Select } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { api } from '../api/client';
import type { LearningPathSummary } from '../api/client';

const Catalog: React.FC = () => {
  const navigate = useNavigate();
  const [paths, setPaths] = useState<LearningPathSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [tagFilter, setTagFilter] = useState<string[]>([]);

  useEffect(() => {
    api.listPaths().then(setPaths).finally(() => setLoading(false));
  }, []);

  if (loading) return <Spin size="large" style={{ display: 'block', marginTop: 100 }} />;

  const allTags = [...new Set(paths.flatMap((p) => p.tags || []))].sort();

  const filtered = paths.filter((p) => {
    const matchesSearch =
      !search ||
      p.title.toLowerCase().includes(search.toLowerCase()) ||
      p.description.toLowerCase().includes(search.toLowerCase());
    const matchesTags =
      tagFilter.length === 0 || tagFilter.every((t) => p.tags?.includes(t));
    return matchesSearch && matchesTags;
  });

  return (
    <div>
      <Typography.Title level={2}>Catalog</Typography.Title>
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={12}>
          <Input
            prefix={<SearchOutlined />}
            placeholder="Search learning paths..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            allowClear
          />
        </Col>
        <Col xs={24} sm={12}>
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
