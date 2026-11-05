// The documentation site: Starlight over the Markdown files of docs/
// (src/content/docs is a link to it). DOCS_SITE and DOCS_BASE set where
// it is published, GitHub Pages of kosatka-cms/kosatka by default.
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { satteri } from '@astrojs/markdown-satteri';
import markdownLinks from './plugins/markdown-links.mjs';

export default defineConfig({
	site: process.env.DOCS_SITE ?? 'https://kosatka-cms.github.io',
	base: process.env.DOCS_BASE ?? '/kosatka',
	trailingSlash: 'always',
	markdown: {
		processor: satteri({ mdastPlugins: [markdownLinks] }),
	},
	integrations: [
		starlight({
			title: 'Kosatka',
			description: 'A general-purpose content management system written in Go.',
			logo: { src: './src/content/docs/assets/logo.svg' },
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/kosatka-cms/kosatka' }],
			editLink: { baseUrl: 'https://github.com/kosatka-cms/kosatka/edit/main/docs/' },
			sidebar: [
				{ label: 'Getting started', items: [{ autogenerate: { directory: 'getting-started' } }] },
				{ label: 'Building a site', items: [{ autogenerate: { directory: 'guides' } }] },
				{ label: 'Themes', items: [{ autogenerate: { directory: 'themes' } }] },
				{
					label: 'Modules',
					items: [
						'modules',
						'modules/using-modules',
						{ label: "The engine's modules", items: [{ autogenerate: { directory: 'modules/builtin' } }] },
						'modules/write-a-module',
						'modules/sdk',
					],
				},
				{ label: 'Reference', items: [{ autogenerate: { directory: 'reference' } }] },
				{ label: 'Contributing', items: [{ autogenerate: { directory: 'contributing' } }] },
			],
		}),
	],
});
