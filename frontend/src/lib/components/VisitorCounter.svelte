<script lang="ts">
	import { visitorCount } from '#lib/visitors/visitors.svelte.js';

	const formatter = new Intl.NumberFormat('en');
</script>

<p class="counter" data-testid="visitor-counter" data-status={visitorCount.status}>
	<span class="value">
		{#if visitorCount.status === 'ready' && visitorCount.count !== null}
			{formatter.format(visitorCount.count)}
		{:else if visitorCount.status === 'unavailable'}
			<span aria-hidden="true">—</span><span class="visually-hidden">Visitor count unavailable</span
			>
		{:else}
			<span role="status" aria-label="Loading visitor count">…</span>
		{/if}
	</span>
	<span class="label">visitors</span>
</p>

<style>
	.counter {
		display: inline-flex;
		align-items: baseline;
		gap: var(--space-2);
		margin: 0;
		color: var(--color-text-muted);
	}

	.value {
		min-width: 2.5ch;
		font-size: 1.5rem;
		font-weight: 700;
		color: var(--color-text);
		font-variant-numeric: tabular-nums;
	}
</style>
