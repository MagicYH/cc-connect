import api from './client';

export interface Subscription {
  id: string;
  project: string;
  chat_id: string;
  chat_name?: string;
  platform: string;
  session_key: string;
  filter?: string;
  exclude_filter?: string;
  prompt: string;
  anchor?: string;
  interval: string;
  concurrency_limit: number;
  timeout_mins: number;
  enabled: boolean;
  last_run?: string;
  last_error?: string;
  consecutive_errors?: number;
  processed_ids?: string[];
  created_at: string;
  updated_at: string;
}

export const listSubscriptions = (project?: string) =>
  api.get<{ subscriptions: Subscription[] }>('/subscription', project ? { project } : undefined);
export const createSubscription = (data: Partial<Subscription>) => api.post<Subscription>('/subscription', data);
export const getSubscription = (id: string) => api.get<Subscription>(`/subscription/${id}`);
export const updateSubscription = (id: string, data: Record<string, any>) => api.patch<Subscription>(`/subscription/${id}`, data);
export const deleteSubscription = (id: string) => api.delete(`/subscription/${id}`);
export const enableSubscription = (id: string) => api.post<Subscription>(`/subscription/${id}/enable`);
export const disableSubscription = (id: string) => api.post<Subscription>(`/subscription/${id}/disable`);
export const runSubscription = (id: string) => api.post<{ id: string; status: string }>(`/subscription/${id}/run`);
