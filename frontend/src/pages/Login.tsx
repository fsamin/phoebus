import React, { useState, useEffect } from 'react';
import { Card, Form, Input, Button, Typography, Alert, Divider, Tabs } from 'antd';
import { FireOutlined, LoginOutlined, SafetyOutlined } from '@ant-design/icons';
import { useAuth } from '../contexts/AuthContext';
import { useNavigate, useSearchParams, Navigate } from 'react-router-dom';

interface AuthProviders {
  local: boolean;
  oidc: boolean;
  ldap: boolean;
}

const Login: React.FC = () => {
  const { user, login } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [providers, setProviders] = useState<AuthProviders | null>(null);
  const [authMode, setAuthMode] = useState<string>('local');

  useEffect(() => {
    fetch('/api/auth/providers')
      .then((r) => r.json())
      .then((p: AuthProviders) => {
        setProviders(p);
        if (p.oidc && !p.local && !p.ldap) {
          // Auto-redirect to OIDC if it's the only provider
          window.location.href = '/api/auth/oidc/redirect';
        } else if (p.local) {
          setAuthMode('local');
        } else if (p.ldap) {
          setAuthMode('ldap');
        }
      });
  }, []);

  if (user) {
    return <Navigate to={searchParams.get('redirect') || '/'} replace />;
  }

  const onFinish = async (values: { username: string; password: string }) => {
    setLoading(true);
    setError('');
    try {
      if (authMode === 'ldap') {
        const resp = await fetch('/api/auth/ldap/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          credentials: 'include',
          body: JSON.stringify(values),
        });
        if (!resp.ok) {
          const data = await resp.json();
          throw new Error(data.error || 'Login failed');
        }
        window.location.href = searchParams.get('redirect') || '/';
      } else {
        await login(values.username, values.password);
        navigate(searchParams.get('redirect') || '/');
      }
    } catch (e) {
      setError((e as Error).message || 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  const loginForm = (
    <Form layout="vertical" onFinish={onFinish}>
      <Form.Item name="username" label="Username" rules={[{ required: true }]}>
        <Input autoFocus />
      </Form.Item>
      <Form.Item name="password" label="Password" rules={[{ required: true }]}>
        <Input.Password />
      </Form.Item>
      <Form.Item>
        <Button type="primary" htmlType="submit" loading={loading} block>
          Sign in
        </Button>
      </Form.Item>
    </Form>
  );

  const tabItems = [];
  if (providers?.local) {
    tabItems.push({ key: 'local', label: 'Local', icon: <LoginOutlined />, children: loginForm });
  }
  if (providers?.ldap) {
    tabItems.push({ key: 'ldap', label: 'LDAP', icon: <SafetyOutlined />, children: loginForm });
  }

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', background: '#f0f2f5' }}>
      <Card style={{ width: 420 }}>
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          <FireOutlined style={{ fontSize: 48, color: '#ff7a45' }} />
          <Typography.Title level={2} style={{ marginTop: 8 }}>Phoebus</Typography.Title>
        </div>
        {error && <Alert message={error} type="error" showIcon style={{ marginBottom: 16 }} />}

        {providers?.oidc && (
          <>
            <Button
              type="default"
              size="large"
              block
              icon={<LoginOutlined />}
              onClick={() => { window.location.href = '/api/auth/oidc/redirect'; }}
              style={{ marginBottom: tabItems.length > 0 ? 0 : 16 }}
            >
              Sign in with SSO
            </Button>
            {tabItems.length > 0 && <Divider>or</Divider>}
          </>
        )}

        {tabItems.length > 1 ? (
          <Tabs
            activeKey={authMode}
            onChange={setAuthMode}
            items={tabItems}
            centered
          />
        ) : tabItems.length === 1 ? (
          loginForm
        ) : null}
      </Card>
    </div>
  );
};

export default Login;
