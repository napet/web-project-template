import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { mockApi } from './helpers/api';

const pages = ['/'];

test.beforeEach(async ({ page }) => {
	await mockApi(page);
});

for (const colorScheme of ['light', 'dark'] as const) {
	test.describe(`${colorScheme} theme`, () => {
		test.use({ colorScheme });

		for (const path of [...pages, '/does-not-exist']) {
			test(`no WCAG AA violations on ${path}`, async ({ page }) => {
				await page.goto(path);
				// wait for the app to finish rendering; only the home page has the API examples
				if (pages.includes(path)) {
					await expect(page.getByTestId('visitor-counter')).toHaveAttribute('data-status', 'ready');
				} else {
					await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
				}
				const results = await new AxeBuilder({ page })
					.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
					.analyze();
				expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
			});
		}
	});
}

test.describe('theme', () => {
	test.use({ colorScheme: 'dark' });

	test('dark system preference renders the dark theme', async ({ page }) => {
		await page.goto('/');
		const background = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
		expect(background).toBe('rgb(18, 20, 23)');
	});
});

test('pages load without Content Security Policy violations', async ({ page }) => {
	await page.addInitScript(() => {
		const violations: string[] = [];
		(window as unknown as { cspViolations: string[] }).cspViolations = violations;
		document.addEventListener('securitypolicyviolation', (event) =>
			violations.push(`${event.violatedDirective} ${event.blockedURI}`)
		);
	});
	for (const path of pages) {
		await page.goto(path);
		await expect(page.getByTestId('health-status')).toHaveAttribute('data-status', 'ok');
		await expect(page.getByTestId('visitor-counter')).toHaveAttribute('data-status', 'ready');
		await expect(page.locator('meta[http-equiv="content-security-policy"]')).toHaveCount(1);
		const violations = await page.evaluate(
			() => (window as unknown as { cspViolations: string[] }).cspViolations
		);
		expect(violations, path).toEqual([]);
	}
});
