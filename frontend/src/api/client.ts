export interface AppInfo {
  appname: string;
  display_name: string;
  description?: string;
  installed: boolean;
  installed_version: string;
  latest_version: string;
  available_version?: string;
  has_update: boolean;
  platform: string;
  release_url: string;
  release_notes: string;
  status: string;
  service_port?: number;
  homepage?: string;
  icon_url?: string;
  updated_at?: string;
}

export interface AppsResponse {
  apps: AppInfo[];
  last_check: string;
}

export interface CheckResponse {
  status: string;
  checked: number;
  updates_available: number;
}

export interface UpdateProgress {
  type?: string;
  step: string;
  progress?: number;
  message?: string;
  new_version?: string;
  app?: string;
  error?: string;
}

export const fetchApps = async (): Promise<AppsResponse> => {
  const response = await fetch('/api/apps');
  if (!response.ok) {
    throw new Error(`Failed to fetch apps: ${response.statusText}`);
  }
  return response.json();
};

export const triggerCheck = async (): Promise<CheckResponse> => {
  const response = await fetch('/api/check', {
    method: 'POST',
  });
  if (!response.ok) {
    throw new Error(`Failed to trigger check: ${response.statusText}`);
  }
  return response.json();
};

export type SSECallback = (event: UpdateProgress) => void;

export interface SSEHandle {
  promise: Promise<void>;
  cancel: () => void;
}

function streamSSE(url: string, onEvent: SSECallback): SSEHandle {
  const controller = new AbortController();

  const promise = (async () => {
    const response = await fetch(url, { method: 'POST', signal: controller.signal });
    if (!response.ok) {
      throw new Error(`Request failed: ${response.statusText}`);
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error('No response body');
    }

    const decoder = new TextDecoder();
    let buffer = '';
    let pendingData = '';

    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            pendingData = line.slice(6);
          } else if (line === '' && pendingData) {
            try {
              onEvent(JSON.parse(pendingData));
            } catch { /* ignore parse errors */ }
            pendingData = '';
          }
        }
      }
    } finally {
      reader.releaseLock();
    }
  })();

  return { promise, cancel: () => controller.abort() };
}

export const installApp = (appname: string, onEvent: SSECallback): SSEHandle => {
  return streamSSE(`/api/apps/${appname}/install`, onEvent);
};

export const updateApp = (appname: string, onEvent: SSECallback): SSEHandle => {
  return streamSSE(`/api/apps/${appname}/update`, onEvent);
};

export const uninstallApp = (appname: string, onEvent: SSECallback): SSEHandle => {
  return streamSSE(`/api/apps/${appname}/uninstall`, onEvent);
};

export interface Settings {
  check_interval_hours: number;
  mirror: string;
  mirror_options?: string[];
}

export interface StatusResponse {
  version?: string;
  platform: string;
}

export interface StoreUpdateInfo {
  current_version: string;
  available_version?: string;
  has_update: boolean;
}

export const fetchSettings = async (): Promise<Settings> => {
  const response = await fetch('/api/settings');
  if (!response.ok) {
    throw new Error(`Failed to fetch settings: ${response.statusText}`);
  }
  return response.json();
};

export const updateSettings = async (settings: Settings): Promise<void> => {
  const response = await fetch('/api/settings', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(settings),
  });
  if (!response.ok) {
    throw new Error(`Failed to update settings: ${response.statusText}`);
  }
};

export const fetchStatus = async (): Promise<StatusResponse> => {
  const response = await fetch('/api/status');
  if (!response.ok) {
    throw new Error(`Failed to fetch status: ${response.statusText}`);
  }
  return response.json();
};

export const fetchStoreUpdate = async (): Promise<StoreUpdateInfo> => {
  const response = await fetch('/api/store-update');
  if (!response.ok) {
    throw new Error(`Failed to fetch store update info: ${response.statusText}`);
  }
  return response.json();
};

export const triggerStoreUpdate = (onEvent: SSECallback): SSEHandle => {
  return streamSSE('/api/store-update', onEvent);
};
