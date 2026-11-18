import { expect, test, type Page } from '@playwright/test';
import { mockApi } from './helpers/api';

const counter = (page: Page) => page.getByTestId('visitor-counter');

test('first visit of the day counts once and records the day', async ({ page }) => {
	const api = await mockApi(page);
	await page.goto('/');

	await expect(counter(page)).toContainText('42');
	await expect.poll(api.increments).toBe(1);
	const day = await page.evaluate(() => localStorage.getItem('visit-day'));
	expect(day).toMatch(/^\d{4}-\d{2}-\d{2}$/);
});

test('later page views the same day do not count again', async ({ page }) => {
	const api = await mockApi(page);
	await page.goto('/');
	await expect.poll(api.increments).toBe(1);
	await expect.poll(() => page.evaluate(() => localStorage.getItem('visit-day'))).not.toBeNull();

	await page.reload();
	await expect(counter(page)).toContainText('42');
	expect(api.increments()).toBe(1);
});

test('a rate-limited increment still records the day', async ({ page }) => {
	await mockApi(page, { incrementStatus: 429 });
	await page.goto('/');
	await expect.poll(() => page.evaluate(() => localStorage.getItem('visit-day'))).not.toBeNull();
});

test('page works when storage throws', async ({ page }) => {
	await page.addInitScript(() => {
		const fail = () => {
			throw new DOMException('blocked', 'SecurityError');
		};
		Storage.prototype.getItem = fail;
		Storage.prototype.setItem = fail;
	});
	const errors: Error[] = [];
	page.on('pageerror', (error) => errors.push(error));
	await mockApi(page);

	await page.goto('/');
	await expect(counter(page)).toContainText('42');
	expect(errors).toEqual([]);
});

test('loading indicator shows while the API is slow', async ({ page }) => {
	await mockApi(page, { streamDelayMs: 3000 });
	await page.goto('/');

	await expect(counter(page).getByRole('status', { name: 'Loading visitor count' })).toBeVisible();
	await expect(counter(page)).toHaveAttribute('data-status', 'loading');

	await expect(counter(page)).toContainText('42', { timeout: 10_000 });
});

test('shows a dash after 20 s when the API never answers', async ({ page }) => {
	await page.clock.install();
	const errors: Error[] = [];
	page.on('pageerror', (error) => errors.push(error));
	const api = await mockApi(page, { streamHangs: true });
	await page.goto('/');

	// "loading" is already in the prerendered HTML; wait until the app has started the counter
	await expect.poll(api.streams).toBeGreaterThan(0);
	await expect(counter(page)).toHaveAttribute('data-status', 'loading');
	// The installed clock flows with real time until paused, and the 20 s fallback timer starts
	// before the stream request we waited for. Pause now, and check "still loading" at 15 s so
	// up to 5 s of real time spent before the pause cannot push it past 20 s on a slow machine.
	await page.clock.pauseAt((await page.evaluate(() => Date.now())) + 100);
	await page.clock.runFor(15_000);
	await expect(counter(page)).toHaveAttribute('data-status', 'loading');
	await page.clock.runFor(6_000);
	await expect(counter(page)).toHaveAttribute('data-status', 'unavailable');
	await expect(counter(page)).toContainText('—');
	expect(errors).toEqual([]);
});

test('cold start: loading, then dash, then the number once the API wakes up', async ({ page }) => {
	await page.clock.install();
	const api = await mockApi(page, { streamDelayMs: 4000 });
	await page.goto('/');

	await expect.poll(api.streams).toBeGreaterThan(0);
	await expect(counter(page)).toHaveAttribute('data-status', 'loading');
	await page.clock.runFor(21_000);
	await expect(counter(page)).toContainText('—');
	await expect(counter(page)).toContainText('42', { timeout: 10_000 });
	await expect(counter(page)).toHaveAttribute('data-status', 'ready');
});

test('count updates live without a reload', async ({ page }) => {
	// first connection sends 42 and ends; the reconnect sends 43
	await mockApi(page, { streamCounts: [[42], [43]] });
	await page.goto('/');

	await expect(counter(page)).toContainText('42');
	await expect(counter(page)).toContainText('43', { timeout: 15_000 });
});
