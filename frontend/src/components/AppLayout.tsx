import React from 'react';
import { Layout, Menu, Dropdown, Button, Space, Typography, Spin, Grid } from 'antd';
import {
  BookOutlined,
  DashboardOutlined,
  BarChartOutlined,
  SettingOutlined,
  UserOutlined,
  LogoutOutlined,
  FireOutlined,
  TeamOutlined,
  HeartOutlined,
  MoonOutlined,
  SunOutlined,
  QuestionCircleOutlined,
  ApartmentOutlined,
  NotificationOutlined,
} from '@ant-design/icons';
import { Outlet, useNavigate, useLocation, Navigate, Link } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { useTheme } from '../contexts/ThemeContext';
import { useOnboardingContext } from '../contexts/OnboardingContext';

const { Header, Content } = Layout;

const AppLayout: React.FC = () => {
  const { user, loading, logout } = useAuth();
  const { isDark, toggle } = useTheme();
  const { forceRun } = useOnboardingContext();
  const navigate = useNavigate();
  const location = useLocation();
  const isMobile = !Grid.useBreakpoint().md;
  // Step pages lay out their own full-height view
  const fullBleed = /^\/paths\/[^/]+\/steps\//.test(location.pathname);

  const handleReplayTour = () => {
    if (location.pathname === '/') forceRun('dashboard');
    else if (location.pathname === '/catalog') forceRun('catalog');
  };

  const showTourButton = location.pathname === '/' || location.pathname === '/catalog';

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

  const menuItems: { key: string; icon: React.ReactNode; label: string; children?: { key: string; icon: React.ReactNode; label: string }[] }[] = [
    { key: '/', icon: <DashboardOutlined />, label: 'Dashboard' },
    { key: '/catalog', icon: <BookOutlined />, label: 'Catalog' },
  ];

  if (user.role === 'instructor' || user.role === 'admin') {
    menuItems.push({ key: '/analytics', icon: <BarChartOutlined />, label: 'Analytics' });
  }

  if (user.role === 'admin') {
    menuItems.push({
      key: '/admin',
      icon: <SettingOutlined />,
      label: 'Admin',
      children: [
        { key: '/admin/repositories', icon: <SettingOutlined />, label: 'Repositories' },
        { key: '/admin/users', icon: <TeamOutlined />, label: 'Users' },
        { key: '/admin/dependencies', icon: <ApartmentOutlined />, label: 'Dependencies' },
        { key: '/admin/banner', icon: <NotificationOutlined />, label: 'Banner' },
        { key: '/admin/health', icon: <HeartOutlined />, label: 'Health' },
      ],
    });
  }

  // Path and step pages are reached from the catalog
  const navPath = location.pathname.startsWith('/paths/') ? '/catalog' : location.pathname;
  const currentKey = ['/admin/repositories', '/admin/users', '/admin/dependencies', '/admin/banner', '/admin/health', '/analytics', '/catalog', '/']
    .filter((k) => navPath.startsWith(k))
    .sort((a, b) => b.length - a.length)[0] || '/';

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Header style={{
        display: 'flex', alignItems: 'center', padding: isMobile ? '0 12px' : '0 24px',
        background: 'var(--color-bg-header)',
        borderBottom: isDark ? '2px solid #ff7a45' : 'none',
      }}>
        <Link
          to="/"
          aria-label="Phœbus home"
          style={{ display: 'flex', alignItems: 'center', marginRight: isMobile ? 8 : 32, flexShrink: 0 }}
        >
          <FireOutlined style={{ fontSize: 24, color: '#ff7a45', marginRight: 8 }} />
          {/* Not a heading: each page owns its h1 */}
          {!isMobile && (
            <span style={{ fontSize: 20, fontWeight: 600, lineHeight: 1.4, color: '#fff' }}>
              Phœbus
            </span>
          )}
        </Link>
        <Menu
          theme="dark"
          mode="horizontal"
          selectedKeys={[currentKey]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
          // minWidth 0 lets the menu collapse overflowing items into "…" instead of pushing the buttons out
          style={{ flex: 1, minWidth: 0, background: 'transparent' }}
        />
        {showTourButton && (
          <Button
            type="text"
            icon={<QuestionCircleOutlined />}
            onClick={handleReplayTour}
            style={{ color: '#fff', marginRight: 4 }}
            title="Replay guided tour"
            aria-label="Replay guided tour"
            data-tour="replay-tour"
          />
        )}
        <Button
          type="text"
          icon={isDark ? <SunOutlined /> : <MoonOutlined />}
          onClick={toggle}
          style={{ color: '#fff', marginRight: 8 }}
          title={isDark ? 'Switch to light theme' : 'Switch to dark theme'}
          aria-label={isDark ? 'Switch to light theme' : 'Switch to dark theme'}
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
          <Button
            type="text"
            icon={<UserOutlined />}
            style={{ color: '#fff' }}
            aria-label={isMobile ? `Account: ${user.display_name || user.username}` : undefined}
          >
            {!isMobile && (user.display_name || user.username)}
          </Button>
        </Dropdown>
      </Header>
      <Content style={fullBleed ? undefined : { padding: isMobile ? 16 : '24px 48px' }}>
        <Outlet />
      </Content>
    </Layout>
  );
};

export default AppLayout;
