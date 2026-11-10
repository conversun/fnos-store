import { apiBasePath, apiUrl } from './base';
import { t } from '../lib/i18n';

class GatewayCSRFError extends Error {
  readonly status: number;

  constructor(status: number) {
    super(`${t('无法获取网关安全令牌，请刷新页面后重试')} (HTTP ${status})`);
    this.name = 'GatewayCSRFError';
    this.status = status;
  }
}

type MutationOptions = Omit<RequestInit, 'method'> & {
  readonly method: 'POST' | 'PUT' | 'DELETE';
};

/** Receives an already mount-resolved URL, just like streamSSE. */
export async function mutationFetch(url: string, options: MutationOptions): Promise<Response> {
  if (apiBasePath().replace(/\/+$/, '') !== '/app/fnos-apps-store') {
    return fetch(url, options);
  }

  // A fresh token belongs to the current backend process, including after restarts.
  // Propagate cancellation through both the token request and the operation.
  const response = await fetch(apiUrl('/api/csrf'), {
    cache: 'no-store',
    signal: options.signal,
  });
  if (!response.ok) throw new GatewayCSRFError(response.status);
  const data: unknown = await response.json();
  if (typeof data !== 'object' || data === null || !('csrf_token' in data)
    || typeof data.csrf_token !== 'string' || data.csrf_token.length === 0) {
    throw new GatewayCSRFError(response.status);
  }
  const headers = new Headers(options.headers);
  headers.set('X-FNOS-CSRF-Token', data.csrf_token);
  return fetch(url, { ...options, headers });
}
