export interface AppInfo {
  appname: string;
  display_name: string;
  description?: string;
  installed: boolean;
  installed_version: string;
  latest_version: string;
  available_version?: string;
  has_update: boolean;
  update_ignored?: boolean;
  platform: string;
  release_url: string;
  release_notes: string;
  status: string;
  service_port?: number;
  homepage?: string;
  icon_url?: string;
  updated_at?: string;
  download_count?: number;
  app_type?: string;
  category?: string;
  post_install_note?: string;
}

export interface AppsResponse {
  apps: AppInfo[];
  last_check: string;
  /** false on fnOS builds where an in-store update would destroy the app. */
  upgrade_allowed?: boolean;
  upgrade_blocked_reason?: string;
}

export interface RecommendedApp {
  name: string;
  display_name: string;
  description: string;
  source_url: string;
  github_repo?: string;
  latest_version?: string;
  updated_at?: string;
}

export interface RecommendedAppsResponse {
  apps: RecommendedApp[];
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
  speed?: number;
  downloaded?: number;
  total?: number;
}

export interface AppOperation {
  step: string;
  progress: number;
  message: string;
  cancel?: () => void;
  speed?: number;
  downloaded?: number;
  total?: number;
}

export const fetchApps = async (): Promise<AppsResponse> => {
  const response = await fetch('/api/apps');
  if (!response.ok) {
    throw new Error(`Failed to fetch apps: ${response.statusText}`);
  }
  return response.json();
};

export const fetchRecommended = async (): Promise<RecommendedAppsResponse> => {
  const response = await fetch('/api/recommended');
  if (!response.ok) {
    return { apps: [] };
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

    const dispatchPending = () => {
      if (!pendingData) return;
      try {
        onEvent(JSON.parse(pendingData));
      } catch (e) {
        // Don't silently drop terminal events ('done' / 'error') -- log so
        // we can debug a UI stuck in a spinner. Truncate the raw payload to
        // avoid leaking large/sensitive data into the browser console.
        const preview = pendingData.length > 200
          ? pendingData.slice(0, 200) + `...(+${pendingData.length - 200} chars)`
          : pendingData;
        console.warn('streamSSE: failed to parse event payload', e, 'preview:', preview);
      }
      pendingData = '';
    };

    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const rawLine of lines) {
          // Trim trailing CR so CRLF-style streams (some proxies/servers) parse correctly.
          const line = rawLine.endsWith('\r') ? rawLine.slice(0, -1) : rawLine;
          if (line.startsWith('data: ')) {
            // SSE spec: multiple consecutive data: lines are joined with newline.
            pendingData += (pendingData ? '\n' : '') + line.slice(6);
          } else if (line === '' && pendingData) {
            dispatchPending();
          }
        }
      }
      // EOF flush: if the stream ends after a 'data:' line but BEFORE the
      // blank-line terminator (e.g. server killed mid-event), dispatch what
      // we have. Without this, the final 'done' / 'error' event can be lost,
      // leaving the UI stuck on a spinner.
      buffer += decoder.decode();
      if (buffer) {
        const tail = buffer.endsWith('\r') ? buffer.slice(0, -1) : buffer;
        if (tail.startsWith('data: ')) {
          pendingData += (pendingData ? '\n' : '') + tail.slice(6);
        }
      }
      dispatchPending();
    } finally {
      reader.releaseLock();
    }
  })();

  return { promise, cancel: () => controller.abort() };
}

/** One answer to an app's install wizard. */
export interface WizardParam {
  key: string;
  value: string;
}

/** An app's install-time form, as the app itself declares it. */
export interface AppWizard {
  appname: string;
  version?: string;
  has_wizard: boolean;
  /** Raw fnOS wizard definition; rendered as-is so new field types keep working. */
  content?: WizardStep[];
  install_volume_id?: number;
  error?: string;
}

export interface WizardStep {
  stepTitle?: string;
  items?: WizardItem[];
}

export interface WizardItem {
  type: string;
  field?: string;
  label?: string;
  helpText?: string;
  initValue?: string;
  rules?: { required?: boolean; message?: string; min?: number }[];
}

export const fetchWizard = async (appname: string): Promise<AppWizard> => {
  const r = await fetch(`/api/apps/${appname}/wizard`);
  if (!r.ok) return { appname, has_wizard: false };
  return r.json();
};

export const installApp = (appname: string, onEvent: SSECallback, wizard?: WizardParam[]): SSEHandle => {
  const qs = wizard && wizard.length ? `?wizard=${encodeURIComponent(JSON.stringify(wizard))}` : '';
  return streamSSE(`/api/apps/${appname}/install${qs}`, onEvent);
};

export const updateApp = (appname: string, onEvent: SSECallback): SSEHandle => {
  return streamSSE(`/api/apps/${appname}/update`, onEvent);
};

export const uninstallApp = (appname: string, onEvent: SSECallback): SSEHandle => {
  return streamSSE(`/api/apps/${appname}/uninstall`, onEvent);
};

export const reloadApps = (onEvent: SSECallback): SSEHandle => {
  return streamSSE('/api/apps/reload', onEvent);
};

export interface MirrorOption {
  key: string;
  label: string;
  description: string;
}

export interface VolumeOption {
  index: number;
  path: string;
  total_bytes: number;
  free_bytes: number;
}

export interface Settings {
  check_interval_hours: number;
  mirror: string;
  mirror_options?: MirrorOption[];
  docker_mirror: string;
  docker_mirror_options?: MirrorOption[];
  custom_github_mirror?: string;
  custom_docker_mirror?: string;
  install_volume: number;
  volume_options?: VolumeOption[];
}

export interface MirrorCheckResult {
  key: string;
  label: string;
  latency_ms: number;
  status: 'ok' | 'timeout' | 'error';
}

export interface MirrorCheckResponse {
  github_mirrors: MirrorCheckResult[];
  docker_mirrors: MirrorCheckResult[];
}

export const checkMirrors = async (type?: 'github' | 'docker'): Promise<MirrorCheckResponse> => {
  const params = type ? `?type=${type}` : '';
  const response = await fetch(`/api/mirrors/check${params}`, { method: 'POST' });
  if (!response.ok) {
    throw new Error(`Failed to check mirrors: ${response.statusText}`);
  }
  return response.json();
};


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

export const updateSettings = async (settings: { check_interval_hours: number; mirror: string; docker_mirror: string; custom_github_mirror?: string; custom_docker_mirror?: string; install_volume: number }): Promise<void> => {
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

export const ignoreUpdate = async (appname: string): Promise<void> => {
  const response = await fetch(`/api/apps/${appname}/ignore-update`, { method: 'PUT' });
  if (!response.ok) {
    throw new Error(`Failed to ignore update: ${response.statusText}`);
  }
};

export const unignoreUpdate = async (appname: string): Promise<void> => {
  const response = await fetch(`/api/apps/${appname}/ignore-update`, { method: 'DELETE' });
  if (!response.ok) {
    throw new Error(`Failed to unignore update: ${response.statusText}`);
  }
};

export interface DiagnosticReport {
  app: string;
  display_name: string;
  version?: string;
  arch: 'x86' | 'ARM';
  app_type?: string;
  failed_step: string;
  error_message: string;
  log_tail: string;
  log_truncated: boolean;
  store_version: string;
  platform: string;
  timestamp: string;
}

export interface DiagnosticResponse {
  report: DiagnosticReport;
  issue_url: string;
}

export async function fetchDiagnostic(app: string, step: string, errorMsg: string): Promise<DiagnosticResponse> {
  const params = new URLSearchParams({ step, error: errorMsg });
  const res = await fetch(`/api/apps/${encodeURIComponent(app)}/diagnostic?${params}`);
  if (!res.ok) throw new Error(`获取诊断信息失败: ${res.status}`);
  return res.json();
}
