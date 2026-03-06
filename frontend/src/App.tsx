import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider, theme as antTheme } from 'antd';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import { ThemeProvider, useTheme } from './contexts/ThemeContext';
import { OnboardingProvider } from './contexts/OnboardingContext';
import AppLayout from './components/AppLayout';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import Catalog from './pages/Catalog';
import PathOverview from './pages/PathOverview';
import StepView from './pages/StepView';
import Repositories from './pages/admin/Repositories';
import RepoForm from './pages/admin/RepoForm';
import Users from './pages/admin/Users';
import Health from './pages/admin/Health';
import SyncLogs from './pages/admin/SyncLogs';
import InstructorSyncLogs from './pages/instructor/InstructorSyncLogs';
import Analytics from './pages/analytics/Analytics';
import PathAnalyticsView from './pages/analytics/PathAnalytics';
import LearnerDetail from './pages/analytics/LearnerDetail';

function RequireRole({ role, children }: { role: string; children: React.ReactNode }) {
  const { user } = useAuth();
  if (!user) return <Navigate to="/login" replace />;
  const roleOrder = ['learner', 'instructor', 'admin'];
  if (roleOrder.indexOf(user.role) < roleOrder.indexOf(role)) {
    return <Navigate to="/" replace />;
  }
  return <>{children}</>;
}

function ThemedApp() {
  const { isDark } = useTheme();
  return (
    <ConfigProvider theme={{
      token: { colorPrimary: '#ff7a45' },
      algorithm: isDark ? antTheme.darkAlgorithm : antTheme.defaultAlgorithm,
    }}>
      <AuthProvider>
        <OnboardingProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/login" element={<Login />} />
            <Route element={<AppLayout />}>
              <Route path="/" element={<Dashboard />} />
              <Route path="/catalog" element={<Catalog />} />
              <Route path="/paths/:pathId" element={<PathOverview />} />
              <Route path="/paths/:pathId/steps/:stepId" element={<StepView />} />
              <Route path="/analytics" element={<RequireRole role="instructor"><Analytics /></RequireRole>} />
              <Route path="/analytics/paths/:pathId" element={<RequireRole role="instructor"><PathAnalyticsView /></RequireRole>} />
              <Route path="/analytics/learners/:learnerId" element={<RequireRole role="instructor"><LearnerDetail /></RequireRole>} />
              <Route path="/instructor/repositories/:repoId/sync-logs" element={<RequireRole role="instructor"><InstructorSyncLogs /></RequireRole>} />
              <Route path="/admin/repositories" element={<RequireRole role="admin"><Repositories /></RequireRole>} />
              <Route path="/admin/repositories/new" element={<RequireRole role="admin"><RepoForm /></RequireRole>} />
              <Route path="/admin/repositories/:repoId/edit" element={<RequireRole role="admin"><RepoForm /></RequireRole>} />
              <Route path="/admin/repositories/:repoId/sync-logs" element={<RequireRole role="admin"><SyncLogs /></RequireRole>} />
              <Route path="/admin/users" element={<RequireRole role="admin"><Users /></RequireRole>} />
              <Route path="/admin/health" element={<RequireRole role="admin"><Health /></RequireRole>} />
            </Route>
          </Routes>
        </BrowserRouter>
        </OnboardingProvider>
      </AuthProvider>
    </ConfigProvider>
  );
}

function App() {
  return (
    <ThemeProvider>
      <ThemedApp />
    </ThemeProvider>
  );
}

export default App;
