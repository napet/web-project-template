<script lang="ts">
	import { checkHealth, type HealthResult } from '#lib/health/health.js';
	import { onMount } from 'svelte';

	let result = $state<HealthResult | null>(null);

	onMount(() => {
		void checkHealth().then((value) => (result = value));
	});

	const status = $derived(result === null ? 'loading' : result.ok ? 'ok' : 'error');
</script>

<p class="health" data-testid="health-status" data-status={status} role="status">
	<span class="dot" aria-hidden="true"></span>
	{#if result === null}
		Checking API…
	{:else if result.ok}
		API: {result.message}
	{:else}
		API: {result.error}
	{/if}
</p>

<style>
	.health {
		display: inline-flex;
		align-items: center;
		gap: var(--space-2);
		margin: 0;
		font-weight: 600;
	}

	.dot {
		width: 0.75rem;
		height: 0.75rem;
		border-radius: 50%;
		background: var(--color-text-muted);
	}

	[data-status='ok'] .dot {
		background: var(--color-success);
	}

	[data-status='error'] .dot {
		background: var(--color-error);
	}
</style>
