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
  status: string;
  service_port?: number;
  homepage?: string;
  icon_url?: string;
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

export const installApp = async (appname: string): Promise<void> => {
  const response = await fetch(`/api/apps/${appname}/install`, {
    method: 'POST',
  });
  if (!response.ok) {
    throw new Error(`Failed to install app: ${response.statusText}`);
  }
};

export const updateApp = async (appname: string): Promise<void> => {
  const response = await fetch(`/api/apps/${appname}/update`, {
    method: 'POST',
  });
  if (!response.ok) {
    throw new Error(`Failed to update app: ${response.statusText}`);
  }
};

export const uninstallApp = async (appname: string): Promise<void> => {
  const response = await fetch(`/api/apps/${appname}/uninstall`, {
    method: 'POST',
  });
  if (!response.ok) {
    throw new Error(`Failed to uninstall app: ${response.statusText}`);
  }
};

export const getSSEEventSource = (): EventSource => {
  return new EventSource('/api/events');
};
