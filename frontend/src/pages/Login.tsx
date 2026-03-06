import React, { useState, useEffect } from 'react';
import { Card, Form, Input, Button, Typography, Alert, Divider, Tabs } from 'antd';
import { FireOutlined, LoginOutlined, SafetyOutlined, UserAddOutlined } from '@ant-design/icons';
import { useAuth } from '../contexts/AuthContext';
import { useNavigate, useSearchParams, Navigate } from 'react-router-dom';
import { api } from '../api/client';
import { usePageTitle } from '../hooks/usePageTitle';

interface AuthProviders {
  local: boolean;
  oidc: boolean;
  ldap: boolean;
  proxy: boolean;
}

const Login: React.FC = () => {
  usePageTitle('Login');
  const { user, login } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [providers, setProviders] = useState<AuthProviders | null>(null);
  const [authMode, setAuthMode] = useState<string>('local');
  const [showRegister, setShowRegister] = useState(false);

  useEffect(() => {
    fetch('/api/auth/providers')
      .then((r) => r.json())
      .then((p: AuthProviders) => {
        setProviders(p);
        // If proxy auth is enabled, try to detect if already authenticated
        if (p.proxy) {
          fetch('/api/me', { credentials: 'include' })
            .then((r) => { if (r.ok) window.location.href = redirect; });
        }
        if (p.oidc && !p.local && !p.ldap && !p.proxy) {
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

  const redirect = searchParams.get('redirect') || '/';

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
        window.location.href = redirect;
      } else {
        await login(values.username, values.password);
        navigate(redirect);
      }
    } catch (e) {
      setError((e as Error).message || 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  const onRegister = async (values: { username: string; display_name: string; email?: string; password: string; confirm: string }) => {
    if (values.password !== values.confirm) {
      setError('Passwords do not match');
      return;
    }
    setLoading(true);
    setError('');
    try {
      await api.register({
        username: values.username,
        display_name: values.display_name,
        email: values.email,
        password: values.password,
      });
      window.location.href = redirect;
    } catch (e) {
      setError((e as Error).message || 'Registration failed');
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
      {providers?.local && (
        <div style={{ textAlign: 'center' }}>
          <Typography.Text type="secondary">
            Don't have an account?{' '}
            <Typography.Link onClick={() => { setShowRegister(true); setError(''); }}>Create one</Typography.Link>
          </Typography.Text>
        </div>
      )}
    </Form>
  );

  const registerForm = (
    <Form layout="vertical" onFinish={onRegister}>
      <Typography.Title level={4} style={{ textAlign: 'center', marginBottom: 16 }}>
        <UserAddOutlined /> Create your account
      </Typography.Title>
      <Form.Item name="username" label="Username" rules={[{ required: true, min: 4, max: 32 }]}>
        <Input autoFocus />
      </Form.Item>
      <Form.Item name="display_name" label="Display Name" rules={[{ required: true }]}>
        <Input />
      </Form.Item>
      <Form.Item name="email" label="Email">
        <Input type="email" />
      </Form.Item>
      <Form.Item name="password" label="Password" rules={[{ required: true, min: 8 }]}>
        <Input.Password />
      </Form.Item>
      <Form.Item name="confirm" label="Confirm Password" rules={[{ required: true }]}>
        <Input.Password />
      </Form.Item>
      <Form.Item>
        <Button type="primary" htmlType="submit" loading={loading} block>
          Create Account
        </Button>
      </Form.Item>
      <div style={{ textAlign: 'center' }}>
        <Typography.Text type="secondary">
          Already have an account?{' '}
          <Typography.Link onClick={() => { setShowRegister(false); setError(''); }}>Sign in</Typography.Link>
        </Typography.Text>
      </div>
    </Form>
  );

  const tabItems = [];
  if (providers?.local) {
    tabItems.push({ key: 'local', label: 'Local', icon: <LoginOutlined />, children: showRegister ? registerForm : loginForm });
  }
  if (providers?.ldap) {
    tabItems.push({ key: 'ldap', label: 'LDAP', icon: <SafetyOutlined />, children: loginForm });
  }

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', background: 'var(--color-bg-login)' }}>
      <Card style={{ width: 420 }}>
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          <FireOutlined style={{ fontSize: 48, color: 'var(--color-primary)' }} />
          <Typography.Title level={2} style={{ marginTop: 8 }}>Phoebus</Typography.Title>
        </div>
        {error && <Alert message={error} type="error" showIcon style={{ marginBottom: 16 }} />}

        {providers?.proxy && !providers?.local && !providers?.ldap && !providers?.oidc && (
          <Alert
            message="SSO Authentication"
            description="This platform uses your organization's single sign-on. If you are not redirected automatically, please contact your administrator."
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
          />
        )}

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
            onChange={(key) => { setAuthMode(key); setShowRegister(false); setError(''); }}
            items={tabItems}
            centered
          />
        ) : tabItems.length === 1 ? (
          showRegister ? registerForm : loginForm
        ) : null}
      </Card>
    </div>
  );
};

export default Login;
