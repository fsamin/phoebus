const BASE = '/api';

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(BASE + url, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    ...options,
  });
  if (res.status === 401) {
    if (!window.location.pathname.startsWith('/login')) {
      window.location.href = '/login?redirect=' + encodeURIComponent(window.location.pathname);
    }
    throw new Error('Unauthorized');
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  if (res.status === 204 || res.headers.get('content-length') === '0') {
    return undefined as T;
  }
  return res.json();
}

export const api = {
  // Auth
  login: (username: string, password: string) =>
    request<{ user: User }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  register: (data: { username: string; display_name: string; email?: string; password: string }) =>
    request<{ user: User }>('/auth/register', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  logout: () => request<{ status: string }>('/auth/logout', { method: 'POST' }),
  me: () => request<User>('/me'),

  // Learning paths
  listPaths: () => request<LearningPathSummary[]>('/learning-paths'),
  listPathDependencies: () => request<PathDependenciesResponse>('/learning-paths/dependencies'),
  getPath: (id: string) => request<LearningPathDetail>(`/learning-paths/${id}`),
  getStep: (pathId: string, stepId: string) =>
    request<StepDetail>(`/learning-paths/${pathId}/steps/${stepId}`),
  listCompetencies: () => request<Competency[]>('/competencies'),

  // Progress
  getProgress: (pathId?: string) =>
    request<Progress[]>(`/progress${pathId ? `?learning_path_id=${pathId}` : ''}`),
  updateProgress: (stepId: string, status: 'in_progress' | 'completed') =>
    request<Progress>('/progress', {
      method: 'POST',
      body: JSON.stringify({ step_id: stepId, status }),
    }),

  // Exercises
  submitAttempt: (stepId: string, body: Record<string, unknown>) =>
    request<Record<string, unknown>>(`/exercises/${stepId}/attempt`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  resetExercise: (stepId: string) =>
    request<Progress>(`/exercises/${stepId}/reset`, { method: 'POST' }),
  getAttempts: (stepId: string) =>
    request<ExerciseAttempt[]>(`/exercises/${stepId}/attempts`),

  // Admin
  listRepos: () => request<Array<GitRepository & { path_titles: string[]; owners: RepoOwner[] }>>('/admin/repos'),
  getRepo: (id: string) => request<GitRepository & { owners: RepoOwner[] }>(`/admin/repos/${id}`),
  listInstructorUsers: () => request<RepoOwner[]>('/admin/instructor-users'),
  createRepo: (data: RepoInput) =>
    request<GitRepository>('/admin/repos', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  updateRepo: (id: string, data: RepoInput) =>
    request<GitRepository>(`/admin/repos/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  deleteRepo: (id: string) =>
    request<void>(`/admin/repos/${id}`, { method: 'DELETE' }),
  syncRepo: (id: string) =>
    request<{ status: string }>(`/admin/repos/${id}/sync`, { method: 'POST' }),
  syncLogs: (id: string) =>
    request<SyncLog[]>(`/admin/repos/${id}/sync-logs`),
  syncJobLogs: (repoId: string, jobId: string) =>
    request<SyncJobLogEntry[]>(`/admin/repos/${repoId}/sync-logs/${jobId}`),
  listUsers: (page = 1, perPage = 20) =>
    request<{ users: Array<User & { completed_paths: number }>; total: number; page: number; per_page: number }>(`/admin/users?page=${page}&per_page=${perPage}`),
  createUser: (data: { username: string; display_name: string; email?: string; role: string; password: string }) =>
    request<User>('/admin/users', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  sshPublicKey: () => request<{ public_key: string }>('/admin/ssh-public-key'),
  listManualDependencies: () => request<ManualDependency[]>('/admin/dependencies'),
  createDependency: (sourcePathId: string, targetPathId: string) =>
    request<PathDependencyRecord>('/admin/dependencies', {
      method: 'POST',
      body: JSON.stringify({ source_path_id: sourcePathId, target_path_id: targetPathId }),
    }),
  deleteDependency: (depId: string) =>
    request<void>(`/admin/dependencies/${depId}`, { method: 'DELETE' }),

  // Instructor repos
  instructorListRepos: () => request<Array<GitRepository & { path_titles: string[] }>>('/instructor/repos'),
  instructorSyncRepo: (id: string) => request<{ status: string }>(`/instructor/repos/${id}/sync`, { method: 'POST' }),
  instructorSyncLogs: (id: string) => request<SyncLog[]>(`/instructor/repos/${id}/sync-logs`),
  instructorSyncJobLogs: (repoId: string, jobId: string) => request<SyncJobLogEntry[]>(`/instructor/repos/${repoId}/sync-logs/${jobId}`),

  // Onboarding
  getOnboarding: () => request<Record<string, boolean>>('/me/onboarding'),
  markOnboardingSeen: (tour: string) =>
    request<{ status: string }>('/me/onboarding', {
      method: 'PATCH',
      body: JSON.stringify({ tour }),
    }),
  resetOnboarding: () =>
    request<{ status: string }>('/me/onboarding', { method: 'DELETE' }),

  listRepoPaths: (repoId: string) =>
    request<RepoLearningPath[]>(`/admin/repos/${repoId}/paths`),
  toggleRepoPath: (repoId: string, pathId: string, enabled: boolean) =>
    request<{ status: string; enabled: boolean }>(`/admin/repos/${repoId}/paths/${pathId}`, {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    }),
};

// --- Types ---

export interface User {
  id: string;
  username: string;
  email?: string;
  display_name: string;
  role: 'learner' | 'instructor' | 'admin';
  active: boolean;
  last_login_at?: string;
  created_at: string;
  role_locked?: boolean;
}

export interface LearningPathSummary {
  id: string;
  slug: string;
  title: string;
  description: string;
  icon?: string;
  tags: string[];
  estimated_duration?: string;
  prerequisites?: string[];
  competencies_provided: string[];
  prerequisites_met: boolean;
  module_count: number;
  step_count: number;
  progress_total?: number;
  progress_completed?: number;
  owners: string[];
}

export interface LearningPathDetail {
  id: string;
  slug: string;
  title: string;
  description: string;
  icon?: string;
  tags: string[];
  estimated_duration?: string;
  prerequisites?: string[];
  modules: ModuleWithSteps[];
}

export interface ModuleWithSteps {
  id: string;
  slug: string;
  title: string;
  description: string;
  competencies: string[];
  position: number;
  steps: StepSummary[];
}

export interface StepSummary {
  id: string;
  slug: string;
  title: string;
  type: 'lesson' | 'quiz' | 'terminal-exercise' | 'code-exercise';
  estimated_duration?: string;
  position: number;
}

export interface StepDetail {
  id: string;
  slug: string;
  module_id: string;
  title: string;
  type: 'lesson' | 'quiz' | 'terminal-exercise' | 'code-exercise';
  estimated_duration?: string;
  content_md: string;
  exercise_data?: Record<string, unknown>;
  codebase_files?: CodebaseFile[];
  position: number;
}

export interface CodebaseFile {
  id: string;
  file_path: string;
  content: string;
  language: string;
}

export interface Progress {
  id: string;
  user_id: string;
  step_id: string;
  status: 'not_started' | 'in_progress' | 'completed';
  completed_at?: string;
}

export interface ExerciseAttempt {
  id: string;
  step_id: string;
  answers: Record<string, unknown>;
  is_correct: boolean;
  created_at: string;
}

export interface GitRepository {
  id: string;
  clone_url: string;
  branch: string;
  auth_type: string;
  webhook_uuid: string;
  sync_status: string;
  sync_error?: string;
  last_synced_at?: string;
  created_at: string;
}

export interface RepoLearningPath {
  id: string;
  title: string;
  description: string;
  enabled: boolean;
  module_count: number;
  step_count: number;
}

export interface RepoInput {
  clone_url: string;
  branch: string;
  auth_type: string;
  credentials?: string;
  owner_ids?: string[];
}

export interface RepoOwner {
  id: string;
  username: string;
  display_name: string;
}

export interface SyncLog {
  id: string;
  repo_id: string;
  status: string;
  error: string | null;
  attempts: number;
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
}

export interface SyncJobLogEntry {
  timestamp: string;
  level: string;
  message: string;
  fields?: Record<string, unknown>;
}

export interface Competency {
  name: string;
  learning_path_ids: string[];
}

export interface DependencyEdge {
  source: string;
  target: string;
  type: 'auto' | 'manual' | 'yaml';
  competencies?: string[];
}

export interface PathDependenciesResponse {
  edges: DependencyEdge[];
}

export interface ManualDependency {
  id: string;
  source_path_id: string;
  target_path_id: string;
  dep_type: string;
  created_at: string;
  source_title: string;
  target_title: string;
}

export interface PathDependencyRecord {
  id: string;
  source_path_id: string;
  target_path_id: string;
  dep_type: string;
  created_at: string;
}
