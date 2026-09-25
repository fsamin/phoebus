import React, { useEffect, useState } from 'react';
import { Typography, Table, Tag, Breadcrumb, Input, Segmented, Tooltip, Progress as AntProgress } from 'antd';
import type { SortOrder } from 'antd/es/table/interface';
import { SearchOutlined } from '@ant-design/icons';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { usePageTitle } from '../../hooks/usePageTitle';

interface LearnerRow {
  user_id: string;
  username: string;
  display_name: string;
  enrolled_paths: number;
  completed_paths: number;
  stuck_steps: number;
  last_activity: string | null;
  progress_rate: number | null;
  first_try_rate: number | null;
  inactive: boolean;
}

interface LearnersResponse {
  learners: LearnerRow[];
  total: number;
  inactive_days: number;
  stuck_days: number;
}

const PAGE_SIZE = 20;

const relativeTime = (iso: string) => {
  const days = Math.floor((Date.now() - new Date(iso).getTime()) / 86_400_000);
  if (days < 1) return 'Today';
  if (days === 1) return 'Yesterday';
  return `${days}d ago`;
};

const LearnersAnalytics: React.FC = () => {
  usePageTitle('Learners');
  const navigate = useNavigate();
  // List state lives in the URL so that going back from a learner restores it.
  const [params, setParams] = useSearchParams();
  const q = params.get('q') ?? '';
  const status = params.get('status') ?? '';
  const sort = params.get('sort') ?? 'name';
  const order = params.get('order') === 'desc' ? 'desc' : 'asc';
  const page = Number(params.get('page')) || 1;

  const [searchInput, setSearchInput] = useState(q);
  const [data, setData] = useState<LearnersResponse | null>(null);
  const [loading, setLoading] = useState(true);

  // Every filter change starts back on the first page.
  const update = (patch: Record<string, string>) => {
    const next = new URLSearchParams(params);
    for (const [k, v] of Object.entries({ page: '', ...patch })) {
      if (v) next.set(k, v); else next.delete(k);
    }
    setParams(next, { replace: true });
  };

  // Debounce search 300ms
  useEffect(() => {
    const t = setTimeout(() => {
      if (searchInput.trim() !== q) update({ q: searchInput.trim() });
    }, 300);
    return () => clearTimeout(t);
  }, [searchInput]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    setLoading(true);
    const query = new URLSearchParams({ page: String(page), per_page: String(PAGE_SIZE), sort, order });
    if (q) query.set('q', q);
    if (status) query.set('status', status);
    fetch(`/api/analytics/learners?${query}`, { credentials: 'include' })
      .then((r) => r.json())
      .then(setData)
      .finally(() => setLoading(false));
  }, [q, status, sort, order, page]);

  const sortOrderOf = (key: string): SortOrder => (sort === key ? (order === 'desc' ? 'descend' : 'ascend') : null);
  const percent = (v: number | null) => (v === null ? '—' : `${Math.round(v)}%`);

  return (
    <div>
      <Breadcrumb items={[
        { title: <Link to="/analytics">Analytics</Link> },
        { title: 'Learners' },
      ]} style={{ marginBottom: 16 }} />

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 12, marginBottom: 16 }}>
        <Typography.Title level={2} style={{ margin: 0 }}>Learners</Typography.Title>
        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
          <Segmented
            value={status}
            onChange={(v) => update({ status: String(v) })}
            options={[
              { value: '', label: 'All' },
              { value: 'not_started', label: 'Not started' },
              { value: 'inactive', label: 'Inactive' },
              { value: 'stuck', label: 'Stuck' },
            ]}
          />
          <Input
            prefix={<SearchOutlined />}
            placeholder="Search learners..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            allowClear
            style={{ width: 260 }}
          />
        </div>
      </div>

      <Table<LearnerRow>
        dataSource={data?.learners ?? []}
        rowKey="user_id"
        loading={loading}
        size="small"
        onRow={(record) => ({ onClick: () => navigate(`/analytics/learners/${record.user_id}`), style: { cursor: 'pointer' } })}
        onChange={(pagination, _filters, sorter) => {
          const s = Array.isArray(sorter) ? sorter[0] : sorter;
          const sortKey = s.order ? String(s.columnKey) : 'name';
          const sortOrder = s.order === 'descend' ? 'desc' : 'asc';
          if (sortKey !== sort || sortOrder !== order) {
            update({ sort: sortKey, order: sortOrder });
          } else {
            update({ page: pagination.current && pagination.current > 1 ? String(pagination.current) : '' });
          }
        }}
        pagination={{
          current: page,
          pageSize: PAGE_SIZE,
          total: data?.total ?? 0,
          showSizeChanger: false,
          showTotal: (t) => `${t} learners`,
        }}
        columns={[
          {
            key: 'name',
            title: 'Learner',
            sorter: true,
            sortOrder: sortOrderOf('name'),
            render: (_: unknown, r) => (
              <div>
                <Typography.Text strong>{r.display_name || r.username}</Typography.Text>
                {r.display_name && <Typography.Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>{r.username}</Typography.Text>}
              </div>
            ),
          },
          {
            key: 'completed_paths',
            title: 'Paths completed',
            width: 150,
            sorter: true,
            sortOrder: sortOrderOf('completed_paths'),
            render: (_: unknown, r) => `${r.completed_paths} / ${r.enrolled_paths}`,
          },
          {
            key: 'progress',
            title: 'Progress',
            width: 180,
            sorter: true,
            sortOrder: sortOrderOf('progress'),
            render: (_: unknown, r) => r.progress_rate === null ? '—' : <AntProgress percent={Math.round(r.progress_rate)} size="small" />,
          },
          {
            key: 'first_try_rate',
            title: <Tooltip title="Share of exercises passed at the first attempt">First-try success</Tooltip>,
            width: 150,
            sorter: true,
            sortOrder: sortOrderOf('first_try_rate'),
            render: (_: unknown, r) => percent(r.first_try_rate),
          },
          {
            key: 'last_activity',
            title: 'Last activity',
            width: 130,
            sorter: true,
            sortOrder: sortOrderOf('last_activity'),
            render: (_: unknown, r) => r.last_activity ? relativeTime(r.last_activity) : '—',
          },
          {
            key: 'status',
            title: 'Status',
            width: 200,
            render: (_: unknown, r) => (
              <>
                {r.enrolled_paths === 0 && <Tag>Not started</Tag>}
                {r.inactive && (
                  <Tooltip title={`No activity for more than ${data?.inactive_days} days`}><Tag color="orange">Inactive</Tag></Tooltip>
                )}
                {r.stuck_steps > 0 && (
                  <Tooltip title={`${r.stuck_steps} step(s) started more than ${data?.stuck_days} days ago and not completed`}>
                    <Tag color="red">Stuck</Tag>
                  </Tooltip>
                )}
              </>
            ),
          },
        ]}
      />
    </div>
  );
};

export default LearnersAnalytics;
