import { defineEnvVars } from '@sveltejs/kit/env';

const withoutTrailingSlash = (value: string) => value.replace(/\/+$/, '');

export const variables = defineEnvVars({
	PUBLIC_API_URL: {
		public: true,
		static: true,
		description: 'Base URL of the backend API, e.g. https://api.example.com',
		schema: (value) => {
			if (!value) throw new Error('PUBLIC_API_URL is required (e.g. http://localhost:8080)');
			return withoutTrailingSlash(value);
		}
	}
});
