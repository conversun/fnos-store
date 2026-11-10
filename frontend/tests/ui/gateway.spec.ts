import { expect, test } from '@playwright/test';

const prefix = '/app/fnos-apps-store';

for (const mount of ['', prefix]) {
  test(`mutations and streamed installs work at ${mount || 'standalone root'}`, async ({ page }) => {
    let token = '';
    let tokenReads = 0;
    let installed = false;
    let interval = 24;
    await page.route(url => url.pathname.startsWith(`${mount}/api/`), async route => {
      const request = route.request();
      const path = new URL(request.url()).pathname.slice(mount.length);
      if (path === '/api/csrf') {
        tokenReads += 1;
        token = `fixture-token-${tokenReads}`;
        return route.fulfill({ json: { csrf_token: token } });
      }
      if (request.method() !== 'GET') {
        const provided = request.headers()['x-fnos-csrf-token'];
        if ((mount && (!token || provided !== token)) || (!mount && provided)) {
          return route.fulfill({ status: 403, json: { error: 'CSRF token rejected' } });
        }
        // Rotate after every operation: a stale cached token cannot pass the next one.
        token = '';
      }
      switch (path) {
        case '/api/apps':
          return route.fulfill({ json: {
            apps: [{
              appname: 'gateway-app', display_name: 'Gateway app', installed,
              installed_version: installed ? '1.0' : '', latest_version: '1.0',
              status: installed ? 'running' : 'not_installed', has_update: false,
              app_type: 'native', platform: 'x86, arm', release_url: '', release_notes: '',
            }],
            last_check: '2026-11-10T10:00:00Z',
          } });
        case '/api/settings':
          if (request.method() === 'PUT') interval = request.postDataJSON().check_interval_hours;
          return route.fulfill({ json: {
            check_interval_hours: interval, mirror: 'direct', docker_mirror: 'direct', install_volume: 0,
          } });
        case '/api/store-update':
          return route.fulfill({ json: { current_version: '1.9.10', has_update: false } });
        case '/api/recommended':
          return route.fulfill({ json: { apps: [] } });
        case '/api/apps/gateway-app/wizard':
          return route.fulfill({ json: { has_wizard: false } });
        case '/api/apps/gateway-app/install':
          installed = true;
          return route.fulfill({
            contentType: 'text/event-stream',
            body: 'data: {"step":"done","message":"操作完成"}\n\n',
          });
        default:
          return route.fulfill({ status: 404, json: { error: 'Unexpected request' } });
      }
    });
    await page.goto(`${mount}/`);
    const menu = page.getByRole('button', { name: '打开导航', exact: true });
    if (await menu.isVisible()) await menu.click();
    await page.getByRole('button', { name: '设置', exact: true }).click();
    await page.locator('#check-interval').click();
    await page.getByRole('option', { name: '6 小时', exact: true }).click();
    await page.getByRole('button', { name: '保存', exact: true }).click();
    await expect(page.getByRole('dialog')).toHaveCount(0);

    // Read back via the UI, so sending a header alone is not mistaken for success.
    if (await menu.isVisible()) await menu.click();
    await page.getByRole('button', { name: '设置', exact: true }).click();
    await expect(page.locator('#check-interval')).toContainText('6 小时');
    await page.getByRole('button', { name: '取消', exact: true }).click();
    await page.getByRole('button', { name: '安装', exact: true }).click();
    await expect(page.getByText('安装成功', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: '确定', exact: true }).click();
    await expect(page.getByRole('button', { name: '卸载 Gateway app', exact: true })).toBeVisible();
    expect(tokenReads).toBe(mount ? 2 : 0);
  });
}

test('a denied gateway token prevents the mutation', async ({ page }) => {
  let mutationSent = false;
  await page.route(url => url.pathname.startsWith(`${prefix}/api/`), async route => {
    const path = new URL(route.request().url()).pathname.slice(prefix.length);
    if (route.request().method() !== 'GET') mutationSent = true;
    if (path === '/api/csrf') return route.fulfill({ status: 403, json: { error: 'Admin required' } });
    if (path === '/api/apps') return route.fulfill({ json: { apps: [], last_check: '2026-11-10T10:00:00Z' } });
    if (path === '/api/store-update') return route.fulfill({ json: { current_version: '1.9.10', has_update: false } });
    return route.fulfill({ json: { apps: [] } });
  });
  await page.goto(`${prefix}/`);
  await page.getByRole('button', { name: /^(立即检查|检查更新)$/ }).click();
  await expect(page.getByText('检查更新失败', { exact: true })).toBeVisible();
  expect(mutationSent).toBe(false);
});
