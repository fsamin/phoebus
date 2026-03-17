import React, { useEffect, useState } from 'react';
import { Table, Button, Select, Typography, Space, Popconfirm, message, Card, Tag } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { api } from '../../api/client';
import type { ManualDependency, LearningPathSummary } from '../../api/client';
import { usePageTitle } from '../../hooks/usePageTitle';

const Dependencies: React.FC = () => {
  usePageTitle('Admin — Dependencies');
  const [deps, setDeps] = useState<ManualDependency[]>([]);
  const [paths, setPaths] = useState<LearningPathSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [sourceId, setSourceId] = useState<string | undefined>();
  const [targetId, setTargetId] = useState<string | undefined>();
  const [creating, setCreating] = useState(false);

  const reload = async () => {
    setLoading(true);
    try {
      const [d, p] = await Promise.all([api.listManualDependencies(), api.listPaths()]);
      setDeps(d);
      setPaths(p);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { reload(); }, []);

  const handleCreate = async () => {
    if (!sourceId || !targetId) return;
    if (sourceId === targetId) {
      message.error('Source and target must be different');
      return;
    }
    setCreating(true);
    try {
      await api.createDependency(sourceId, targetId);
      message.success('Dependency created');
      setSourceId(undefined);
      setTargetId(undefined);
      await reload();
    } catch (err: any) {
      message.error(err.message || 'Failed to create dependency');
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (depId: string) => {
    try {
      await api.deleteDependency(depId);
      message.success('Dependency deleted');
      await reload();
    } catch (err: any) {
      message.error(err.message || 'Failed to delete dependency');
    }
  };

  const pathOptions = paths.map((p) => ({
    label: `${p.icon || ''} ${p.title}`.trim(),
    value: p.id,
  }));

  const columns = [
    {
      title: 'Source (prerequisite)',
      dataIndex: 'source_title',
      key: 'source',
    },
    {
      title: '→',
      key: 'arrow',
      width: 40,
      render: () => '→',
    },
    {
      title: 'Target (depends on source)',
      dataIndex: 'target_title',
      key: 'target',
    },
    {
      title: 'Type',
      dataIndex: 'dep_type',
      key: 'dep_type',
      width: 100,
      render: (type: string) => (
        <Tag color={type === 'manual' ? 'purple' : 'blue'}>{type}</Tag>
      ),
    },
    {
      title: 'Actions',
      key: 'actions',
      width: 80,
      render: (_: unknown, record: ManualDependency) =>
        record.dep_type === 'manual' ? (
          <Popconfirm title="Delete this dependency?" onConfirm={() => handleDelete(record.id)}>
            <Button danger size="small" icon={<DeleteOutlined />} />
          </Popconfirm>
        ) : (
          <Typography.Text type="secondary">YAML</Typography.Text>
        ),
    },
  ];

  return (
    <div>
      <Typography.Title level={2}>Path Dependencies</Typography.Title>
      <Typography.Paragraph type="secondary">
        Manage manual dependencies between learning paths. These appear as edges in the Catalog DAG view.
        Auto-detected dependencies (based on competencies) are not shown here.
      </Typography.Paragraph>

      <Card size="small" style={{ marginBottom: 24 }}>
        <Space wrap>
          <Select
            showSearch
            placeholder="Source path (prerequisite)"
            value={sourceId}
            onChange={setSourceId}
            options={pathOptions}
            style={{ width: 280 }}
            filterOption={(input, option) =>
              (option?.label as string)?.toLowerCase().includes(input.toLowerCase()) ?? false
            }
          />
          <Typography.Text>→</Typography.Text>
          <Select
            showSearch
            placeholder="Target path (depends on source)"
            value={targetId}
            onChange={setTargetId}
            options={pathOptions}
            style={{ width: 280 }}
            filterOption={(input, option) =>
              (option?.label as string)?.toLowerCase().includes(input.toLowerCase()) ?? false
            }
          />
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleCreate}
            loading={creating}
            disabled={!sourceId || !targetId}
          >
            Add Dependency
          </Button>
        </Space>
      </Card>

      <Table
        columns={columns}
        dataSource={deps}
        rowKey="id"
        loading={loading}
        pagination={false}
        locale={{ emptyText: 'No manual or YAML dependencies defined' }}
      />
    </div>
  );
};

export default Dependencies;
