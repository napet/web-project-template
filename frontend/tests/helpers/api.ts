import type { Page, Route } from '@playwright/test';

// Match the API on any origin, so the tests do not depend on the PUBLIC_API_URL
// the build was made with.
export const HEALTH_URL = '**/api/v1/health';
export const COUNT_URL = '**/api/v1/visitors';
export const STREAM_URL = '**/api/v1/visitors/stream';

/** Body of an SSE response carrying the given counts as `count` events. */
export function sseBody(...counts: number[]): string {
	return 'retry: 5000\n\n' + counts.map((c) => `event: count\ndata: ${c}\n\n`).join('');
}

export interface MockApiOptions {
	/** Status for GET /health (default 200). */
	healthStatus?: number;
	/** Counts sent by successive stream connections; the last one repeats. */
	streamCounts?: number[][];
	/** Status for POST /visitors (default 200). */
	incrementStatus?: number;
	/** Never answer stream requests (simulates a sleeping API). */
	streamHangs?: boolean;
	/** Delay before answering stream requests, in ms. */
	streamDelayMs?: number;
}

export interface MockApi {
	/** Number of POST /visitors requests received. */
	increments: () => number;
	/** Number of stream connections received. */
	streams: () => number;
}

/** Mocks the backend API so tests never depend on a real backend. */
export async function mockApi(page: Page, options: MockApiOptions = {}): Promise<MockApi> {
	const {
		healthStatus = 200,
		streamCounts = [[42]],
		incrementStatus = 200,
		streamHangs = false,
		streamDelayMs = 0
	} = options;
	let increments = 0;
	let streams = 0;

	await page.route(HEALTH_URL, (route: Route) =>
		route.fulfill({
			status: healthStatus,
			contentType: 'application/json',
			headers: { 'Access-Control-Allow-Origin': '*' },
			body: JSON.stringify({ message: healthStatus === 200 ? 'Server OK' : 'down' })
		})
	);

	await page.route(COUNT_URL, async (route: Route) => {
		if (route.request().method() === 'POST') {
			increments++;
			await route.fulfill({
				status: incrementStatus,
				contentType: 'application/json',
				headers: { 'Access-Control-Allow-Origin': '*' },
				body: JSON.stringify(incrementStatus === 200 ? { count: 43 } : { message: 'limited' })
			});
			return;
		}
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			headers: { 'Access-Control-Allow-Origin': '*' },
			body: JSON.stringify({ count: 42 })
		});
	});

	await page.route(STREAM_URL, async (route: Route) => {
		const counts = streamCounts[Math.min(streams, streamCounts.length - 1)];
		streams++;
		if (streamHangs) return; // leave the request pending forever
		if (streamDelayMs) await new Promise((r) => setTimeout(r, streamDelayMs));
		await route.fulfill({
			status: 200,
			contentType: 'text/event-stream',
			headers: { 'Access-Control-Allow-Origin': '*', 'Cache-Control': 'no-cache' },
			body: sseBody(...counts)
		});
	});

	return { increments: () => increments, streams: () => streams };
}
