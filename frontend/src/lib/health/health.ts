import { PUBLIC_API_URL } from '$app/env/public';

const TIMEOUT_MS = 10_000;

export type HealthResult = { ok: true; message: string } | { ok: false; error: string };

/** Calls the backend's liveness endpoint, GET /api/v1/health. */
export async function checkHealth(): Promise<HealthResult> {
	try {
		const response = await fetch(`${PUBLIC_API_URL}/api/v1/health`, {
			signal: AbortSignal.timeout(TIMEOUT_MS)
		});
		if (!response.ok) return { ok: false, error: `HTTP ${response.status}` };
		const body: { message?: unknown } = await response.json();
		return { ok: true, message: String(body.message ?? 'OK') };
	} catch {
		return { ok: false, error: 'API unreachable' };
	}
}
