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
  return res.json();
}

export const api = {
  // Auth
  login: (username: string, password: string) =>
    request<{ user: User }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request<{ status: string }>('/auth/logout', { method: 'POST' }),
  me: () => request<User>('/me'),

  // Learning paths
  listPaths: () => request<LearningPathSummary[]>('/learning-paths'),
  getPath: (id: string) => request<LearningPathDetail>(`/learning-paths/${id}`),
  getStep: (pathId: string, stepId: string) =>
    request<StepDetail>(`/learning-paths/${pathId}/steps/${stepId}`),

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
  listRepos: () => request<GitRepository[]>('/admin/repos'),
  getRepo: (id: string) => request<GitRepository>(`/admin/repos/${id}`),
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
  listUsers: (page = 1, perPage = 20) =>
    request<{ users: User[]; total: number; page: number; per_page: number }>(`/admin/users?page=${page}&per_page=${perPage}`),
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
}

export interface LearningPathSummary {
  id: string;
  title: string;
  description: string;
  icon?: string;
  tags: string[];
  estimated_duration?: string;
  prerequisites?: string[];
  module_count: number;
  step_count: number;
}

export interface LearningPathDetail {
  id: string;
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
  title: string;
  description: string;
  competencies: string[];
  position: number;
  steps: StepSummary[];
}

export interface StepSummary {
  id: string;
  title: string;
  type: 'lesson' | 'quiz' | 'terminal-exercise' | 'code-exercise';
  estimated_duration?: string;
  position: number;
}

export interface StepDetail {
  id: string;
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

export interface RepoInput {
  clone_url: string;
  branch: string;
  auth_type: string;
  credentials?: string;
}
