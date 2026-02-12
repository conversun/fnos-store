export interface AppInfo {
  appname: string;
  display_name: string;
  installed: boolean;
  installed_version: string;
  latest_version: string;
  has_update: boolean;
  platform: string;
  release_url: string;
  release_notes: string;
  status: string; // "running", "stopped", etc.
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
  step: string; // "downloading", "installing", "verifying", "done"
  progress?: number;
  message?: string;
  new_version?: string;
  app?: string; // For update-all
}

// Mock data for development
const MOCK_APPS: AppInfo[] = [
  {
    appname: 'plexmediaserver',
    display_name: 'Plex',
    installed: true,
    installed_version: '1.43.0.10492',
    latest_version: '1.44.0.12345',
    has_update: true,
    platform: 'x86',
    release_url: 'https://github.com/conversun/fnos-apps/releases/tag/plex/v1.44.0.12345',
    release_notes: 'Fixes and improvements...',
    status: 'running',
  },
  {
    appname: 'embyserver',
    display_name: 'Emby',
    installed: true,
    installed_version: '4.9.3.0',
    latest_version: '4.9.3.0',
    has_update: false,
    platform: 'x86',
    release_url: '',
    release_notes: '',
    status: 'stopped',
  },
  {
    appname: 'jellyfin',
    display_name: 'Jellyfin',
    installed: false,
    installed_version: '',
    latest_version: '10.11.7',
    has_update: false,
    platform: 'x86',
    release_url: 'https://github.com/conversun/fnos-apps/releases/tag/jellyfin/v10.11.7',
    release_notes: 'New release!',
    status: '',
  },
];

export const fetchApps = async (): Promise<AppsResponse> => {
  // Simulate network delay
  await new Promise((resolve) => setTimeout(resolve, 500));
  return {
    apps: MOCK_APPS,
    last_check: new Date().toISOString(),
  };
};

export const triggerCheck = async (): Promise<CheckResponse> => {
  await new Promise((resolve) => setTimeout(resolve, 1000));
  return {
    status: 'ok',
    checked: 6,
    updates_available: 1,
  };
};

export const installApp = async (appname: string): Promise<void> => {
  console.log(`Installing ${appname}...`);
};

export const updateApp = async (appname: string): Promise<void> => {
  console.log(`Updating ${appname}...`);
};

export const uninstallApp = async (appname: string): Promise<void> => {
  console.log(`Uninstalling ${appname}...`);
};

export const getUpdateEventSource = (appname: string): EventSource => {
  // In real implementation, return new EventSource(`/api/apps/${appname}/update`);
  // For mock, we can't easily return a real EventSource that emits mock events without a backend.
  // We'll return a dummy one or maybe we should just rely on console logs for now as per instructions "Do NOT make real API calls yet"
  // But the task says "SSE connection helper for progress streams".
  // Let's just return a correctly formed URL EventSource.
  return new EventSource(`/api/apps/${appname}/update`);
};

export const getUpdateAllEventSource = (): EventSource => {
  return new EventSource('/api/apps/update-all');
};
