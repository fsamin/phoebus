import React from 'react';
import { Layout, Menu, Dropdown, Button, Space, Typography, Spin } from 'antd';
import {
  BookOutlined,
  DashboardOutlined,
  BarChartOutlined,
  SettingOutlined,
  UserOutlined,
  LogoutOutlined,
  FireOutlined,
} from '@ant-design/icons';
import { Outlet, useNavigate, useLocation, Navigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

const { Header, Content } = Layout;

const AppLayout: React.FC = () => {
  const { user, loading, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!user) {
    return <Navigate to={`/login?redirect=${encodeURIComponent(location.pathname)}`} replace />;
  }

  const menuItems = [
    { key: '/', icon: <DashboardOutlined />, label: 'Dashboard' },
    { key: '/catalog', icon: <BookOutlined />, label: 'Catalog' },
  ];

  if (user.role === 'instructor' || user.role === 'admin') {
    menuItems.push({ key: '/analytics', icon: <BarChartOutlined />, label: 'Analytics' });
  }

  if (user.role === 'admin') {
    menuItems.push({ key: '/admin/repositories', icon: <SettingOutlined />, label: 'Admin' });
  }

  const currentKey = menuItems
    .map((i) => i.key)
    .filter((k) => location.pathname.startsWith(k))
    .sort((a, b) => b.length - a.length)[0] || '/';

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{ display: 'flex', alignItems: 'center', padding: '0 24px', background: '#001529' }}>
        <div
          style={{ display: 'flex', alignItems: 'center', cursor: 'pointer', marginRight: 32 }}
          onClick={() => navigate('/')}
        >
          <FireOutlined style={{ fontSize: 24, color: '#ff7a45', marginRight: 8 }} />
          <Typography.Title level={4} style={{ margin: 0, color: '#fff' }}>
            Phoebus
          </Typography.Title>
        </div>
        <Menu
          theme="dark"
          mode="horizontal"
          selectedKeys={[currentKey]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
          style={{ flex: 1 }}
        />
        <Dropdown
          menu={{
            items: [
              {
                key: 'info',
                label: (
                  <Space direction="vertical" size={0}>
                    <Typography.Text strong>{user.display_name || user.username}</Typography.Text>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>{user.role}</Typography.Text>
                  </Space>
                ),
                disabled: true,
              },
              { type: 'divider' },
              { key: 'logout', icon: <LogoutOutlined />, label: 'Logout', danger: true },
            ],
            onClick: ({ key }) => { if (key === 'logout') logout(); },
          }}
        >
          <Button type="text" icon={<UserOutlined />} style={{ color: '#fff' }}>
            {user.display_name || user.username}
          </Button>
        </Dropdown>
      </Header>
      <Content style={{ padding: '24px 48px' }}>
        <Outlet />
      </Content>
    </Layout>
  );
};

export default AppLayout;
