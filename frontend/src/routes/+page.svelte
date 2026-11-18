<script lang="ts">
	import HealthStatus from '#lib/components/HealthStatus.svelte';
	import VisitorCounter from '#lib/components/VisitorCounter.svelte';
</script>

<svelte:head>
	<title>Web project template</title>
	<meta name="description" content="SvelteKit frontend talking to a Go backend." />
</svelte:head>

<h1>Web project template</h1>
<p class="lead">
	A static SvelteKit frontend calling a Go API. Replace this page with your own; the two examples
	below show a plain request and a database-backed resource with live updates.
</p>

<section aria-labelledby="health-heading">
	<h2 id="health-heading">Health check</h2>
	<p>
		One request to <code>GET /api/v1/health</code> when the page loads.
	</p>
	<HealthStatus />
</section>

<section aria-labelledby="counter-heading">
	<h2 id="counter-heading">Visitor counter</h2>
	<p>
		Counts this visit once a day with <code>POST /api/v1/visitors</code>, then follows the count
		live over Server-Sent Events from <code>/api/v1/visitors/stream</code>.
	</p>
	<VisitorCounter />
</section>

<style lang="scss">
	.lead {
		color: var(--color-text-muted);
		margin-bottom: var(--space-8);
	}

	section {
		padding-block: var(--space-6);
		border-top: 1px solid var(--color-border);
	}
</style>
