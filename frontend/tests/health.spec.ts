import { expect, test } from '@playwright/test';
import { mockApi } from './helpers/api';

test('shows the health check result', async ({ page }) => {
	await mockApi(page);
	await page.goto('/');

	const health = page.getByTestId('health-status');
	await expect(health).toHaveAttribute('data-status', 'ok');
	await expect(health).toContainText('Server OK');
});

test('shows an error when the health check fails', async ({ page }) => {
	await mockApi(page, { healthStatus: 503 });
	await page.goto('/');

	const health = page.getByTestId('health-status');
	await expect(health).toHaveAttribute('data-status', 'error');
	await expect(health).toContainText('HTTP 503');
});

test('unknown pages show the 404 page', async ({ page }) => {
	await mockApi(page);
	await page.goto('/does-not-exist');
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Page not found');
});
