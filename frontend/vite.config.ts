import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
	// the API origin is the only place the browser may connect to besides this site
	const env = loadEnv(mode, process.cwd(), 'PUBLIC_');
	// cast: the CSP types only accept string literals shaped like a host source
	const apiOrigin = env.PUBLIC_API_URL
		? (new URL(env.PUBLIC_API_URL).origin as `${string}://${string}.${string}`)
		: undefined;

	return {
		plugins: [
			sveltekit({
				compilerOptions: {
					// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
					runes: ({ filename }) =>
						filename.split(/[/\\]/).includes('node_modules') ? undefined : true
				},

				adapter: adapter({
					pages: 'build',
					assets: 'build',
					fallback: '200.html',
					precompress: false,
					strict: true
				}),

				// prerendered pages get the policy as a <meta http-equiv> tag, with hashes for inline scripts
				csp: {
					mode: 'hash',
					directives: {
						'default-src': ['self'],
						'script-src': ['self'],
						'style-src': ['self'],
						// SvelteKit's route announcer (#svelte-announcer) uses an inline style attribute
						'style-src-attr': ['unsafe-inline'],
						'img-src': ['self', 'data:'],
						'connect-src': apiOrigin ? ['self', apiOrigin] : ['self'],
						'base-uri': ['self'],
						'form-action': ['self'],
						'object-src': ['none']
					}
				},

				preprocess: [vitePreprocess({ style: true, script: false })]
			})
		]
	};
});
