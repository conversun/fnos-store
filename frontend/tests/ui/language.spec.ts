import { expect, test, type Page } from '@playwright/test';

const app = {
  appname: 'native-app',
  display_name: 'Native app',
  description: '应用作者提供的说明',
  installed: false,
  installed_version: '',
  latest_version: '1.0.0',
  has_update: false,
  platform: 'x86, arm',
  release_url: '',
  release_notes: '',
  status: 'not_installed',
  app_type: 'native',
} as const;

async function openSettings(page: Page) {
  const menu = page.getByRole('button', { name: /Open navigation|打开导航/ });
  if (await menu.isVisible()) await menu.click();
  await page.getByRole('button', { name: /^(Settings|设置)$/ }).click();
}

test.beforeEach(async ({ page }) => {
  await page.route(url => url.pathname.startsWith('/api/'), async route => {
    const path = new URL(route.request().url()).pathname;
    switch (path) {
      case '/api/apps':
        return route.fulfill({ json: {
          apps: [
            app,
            { ...app, appname: 'docker-app', display_name: 'Docker app', app_type: 'docker' },
            { ...app, appname: 'local-app', display_name: 'Local app', app_type: '' },
          ],
          last_check: '2026-11-10T10:00:00Z',
        } });
      case '/api/settings':
        return route.fulfill({ json: {
          check_interval_hours: 24, mirror: 'direct', docker_mirror: 'direct', install_volume: 0,
        } });
      case '/api/store-update':
        return route.fulfill({ json: { current_version: '1.9.10', has_update: false } });
      case '/api/recommended':
        return route.fulfill({ json: { apps: [] } });
      default:
        return route.fulfill({ json: {} });
    }
  });
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Native app', exact: true })).toBeVisible();
});

test('language choice changes the interface and survives reloads in both directions', async ({ page }) => {
  await openSettings(page);
  await page.getByRole('combobox', { name: '语言 / Language' }).click();
  await page.getByRole('option', { name: 'English', exact: true }).click();

  await expect(page.getByRole('heading', { name: 'Settings', exact: true })).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('lang', 'en');
  await page.reload();
  await expect(page.getByRole('button', { name: 'Install', exact: true }).first()).toBeVisible();
  await openSettings(page);
  await expect(page.getByRole('combobox', { name: '语言 / Language' })).toContainText('English');

  await page.getByRole('combobox', { name: '语言 / Language' }).click();
  await page.getByRole('option', { name: '简体中文', exact: true }).click();
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('lang', 'zh-CN');
  await expect(page.getByRole('button', { name: '安装', exact: true }).first()).toBeVisible();
});

test('installation type labels distinguish native and Docker without guessing missing metadata', async ({ page }) => {
  await openSettings(page);
  await page.getByRole('combobox', { name: '语言 / Language' }).click();
  await page.getByRole('option', { name: 'English', exact: true }).click();
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();

  for (const [name, label] of [['Native app', 'Native'], ['Docker app', 'Docker'], ['Local app', '']] as const) {
    const heading = page.getByRole('heading', { name, exact: true });
    const card = heading.locator('xpath=ancestor::div[contains(@class,"overflow-hidden")][1]');
    const type = card.locator('[title="Installation type"]');
    if (label) await expect(type).toHaveText(label);
    else await expect(type).toHaveCount(0);
    await heading.click();
    const dialog = page.getByRole('dialog');
    if (label) await expect(dialog.locator('[title="Installation type"]')).toHaveText(label);
    else await expect(dialog.getByText('Installation type', { exact: true })).toHaveCount(0);
    await expect(dialog).toContainText('应用作者提供的说明');
    await dialog.getByRole('button', { name: 'Close', exact: true }).click();
  }
});

test('operation failures retain original diagnostics under a localized error heading', async ({ page }) => {
  await openSettings(page);
  await page.getByRole('combobox', { name: '语言 / Language' }).click();
  await page.getByRole('option', { name: 'English', exact: true }).click();
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();
  await page.route('**/api/apps/native-app/wizard', route => route.fulfill({
    json: { appname: 'native-app', has_wizard: false },
  }));
  await page.route('**/api/apps/native-app/install', route => route.fulfill({
    contentType: 'text/event-stream',
    body: 'data: {"step":"error","message":"上游进程失败: exit 1"}\n\n',
  }));

  const card = page.getByRole('heading', { name: 'Native app', exact: true })
    .locator('xpath=ancestor::div[contains(@class,"overflow-hidden")][1]');
  await card.getByRole('button', { name: 'Install', exact: true }).click();

  await expect(page.getByText('App operation failed', { exact: true })).toBeVisible();
  await expect(page.getByText('Original diagnostic details: 上游进程失败: exit 1', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Report', exact: true })).toBeVisible();
});
