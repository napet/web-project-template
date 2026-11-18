<script lang="ts">
	import { page } from '$app/state';
	import { resolve } from '$app/paths';

	const notFound = $derived(page.status === 404);
	const title = $derived(notFound ? 'Page not found' : 'Something went wrong');
</script>

<svelte:head>
	<title>{title}</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<div class="error">
	<p class="status">{page.status}</p>
	<h1>{title}</h1>
	<p>
		{notFound
			? "The page you're looking for doesn't exist or has moved."
			: 'An unexpected error occurred. Please try again later.'}
	</p>
	<p><a href={resolve('/')}>Go to the home page</a></p>
</div>

<style>
	.error {
		padding-block: var(--space-16);
		text-align: center;
	}

	.status {
		margin: 0;
		font-size: 4rem;
		font-weight: 800;
		color: var(--color-accent);
		line-height: 1;
	}
</style>
