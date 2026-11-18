import { PUBLIC_API_URL } from '$app/env/public';

/** After this long without a value the counter shows "—" (and keeps retrying). */
export const FALLBACK_MS = 20_000;
const INCREMENT_TIMEOUT_MS = 10_000;
const RECONNECT_MIN_MS = 5_000;
const RECONNECT_MAX_MS = 60_000;
const VISIT_DAY_KEY = 'visit-day';

export type VisitorStatus = 'loading' | 'ready' | 'unavailable';

/** Local calendar day as YYYY-MM-DD. */
function today(): string {
	const now = new Date();
	const pad = (n: number) => String(n).padStart(2, '0');
	return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}

// Storage can throw (privacy modes, blocked site data); the counter must still work.
function readVisitDay(): string | null {
	try {
		return localStorage.getItem(VISIT_DAY_KEY);
	} catch {
		return null;
	}
}

function writeVisitDay(day: string): void {
	try {
		localStorage.setItem(VISIT_DAY_KEY, day);
	} catch {
		// nothing to do: the visit may be counted again on the next load
	}
}

/**
 * Counts this visit at most once per browser per local day. The day is only
 * recorded after the API accepted (2xx) or rate-limited (429) the increment.
 */
async function countVisit(): Promise<void> {
	const day = today();
	if (readVisitDay() === day) return;
	try {
		// no body and no custom headers: a simple CORS request, so no preflight
		const response = await fetch(`${PUBLIC_API_URL}/api/v1/visitors`, {
			method: 'POST',
			signal: AbortSignal.timeout(INCREMENT_TIMEOUT_MS)
		});
		if (response.ok || response.status === 429) writeVisitDay(day);
	} catch {
		// API asleep or unreachable: try again on the next page load
	}
}

/** Live visitor count, shared by the whole app. Start it once from the root layout. */
class VisitorCount {
	status = $state<VisitorStatus>('loading');
	count = $state<number | null>(null);

	#source: EventSource | null = null;
	#fallbackTimer: ReturnType<typeof setTimeout> | undefined;
	#reconnectTimer: ReturnType<typeof setTimeout> | undefined;
	#reconnectDelay = RECONNECT_MIN_MS;
	#running = false;

	/** Starts counting and streaming; returns a function that stops it. */
	start(): () => void {
		if (this.#running) return () => {};
		this.#running = true;

		void countVisit();
		this.#connect();
		this.#fallbackTimer = setTimeout(() => {
			if (this.status === 'loading') this.status = 'unavailable';
		}, FALLBACK_MS);

		return () => this.#stop();
	}

	#connect(): void {
		const source = new EventSource(`${PUBLIC_API_URL}/api/v1/visitors/stream`);
		this.#source = source;

		source.addEventListener('count', (event) => {
			const value = Number((event as MessageEvent<string>).data);
			if (!Number.isFinite(value)) return;
			this.count = value;
			this.status = 'ready';
			this.#reconnectDelay = RECONNECT_MIN_MS;
		});

		source.addEventListener('error', () => {
			// EventSource retries dropped connections itself, but gives up for good on
			// HTTP errors or CORS failures (e.g. while the API wakes up), so reconnect manually.
			if (source.readyState !== EventSource.CLOSED || !this.#running) return;
			this.#reconnectTimer = setTimeout(() => this.#connect(), this.#reconnectDelay);
			this.#reconnectDelay = Math.min(this.#reconnectDelay * 2, RECONNECT_MAX_MS);
		});
	}

	#stop(): void {
		this.#running = false;
		clearTimeout(this.#fallbackTimer);
		clearTimeout(this.#reconnectTimer);
		this.#source?.close();
		this.#source = null;
	}
}

export const visitorCount = new VisitorCount();
