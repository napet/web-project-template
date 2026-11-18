<script lang="ts">
	import favicon from '#lib/assets/favicon.svg';
	import '#lib/styles/main.scss';
	import { visitorCount } from '#lib/visitors/visitors.svelte.js';
	import { onMount, type Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	// one counter and one stream connection for the whole visit, across client-side navigation
	onMount(() => visitorCount.start());
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<main id="main">
	{@render children()}
</main>

<style lang="scss">
	@use '#lib/styles/partials' as *;

	main {
		@include content-width;
		padding-block: var(--space-12);
	}
</style>
