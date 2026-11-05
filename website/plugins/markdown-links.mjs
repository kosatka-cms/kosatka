// Relative links between the Markdown files of docs/ (`./views.md`,
// `../modules/sdk.md#section`) become links between their pages,
// so the same files read on GitHub and on the site. A link to a file that
// doesn't exist fails the build.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineMdastPlugin } from 'satteri';

const docsRoot = fs.realpathSync(new URL('../../docs', import.meta.url));

// pagePath - a docs file's page, relative to the site's root, with a
// trailing slash: guides/views.md → guides/views/,
// guides/index.md → guides/.
function pagePath(file) {
	const rel = path.relative(docsRoot, file).split(path.sep).join('/');
	const page = rel.replace(/\.mdx?$/, '').replace(/(^|\/)index$/, '');
	return page === '' ? '' : page + '/';
}

// pageLink - the link from the page of file from to that of file to.
function pageLink(from, to) {
	const rel = path.posix.relative(pagePath(from) || '.', pagePath(to) || '.');
	if (rel === '') return './';
	return (rel.startsWith('.') ? rel : './' + rel) + '/';
}

export default defineMdastPlugin({
	name: 'kosatka-docs-links',
	link(node, ctx) {
		if (!ctx.fileURL || /^[a-z][a-z0-9+.-]*:|^\/|^#/i.test(node.url)) return;
		const [target, hash] = node.url.split('#');
		if (!/\.mdx?$/.test(target)) return;
		const from = fs.realpathSync(fileURLToPath(ctx.fileURL));
		if (!from.startsWith(docsRoot + path.sep)) return;
		const to = path.resolve(path.dirname(from), decodeURIComponent(target));
		if (!fs.existsSync(to)) {
			throw new Error(`${path.relative(docsRoot, from)}: a link to a missing file: ${node.url}`);
		}
		ctx.setProperty(node, 'url', pageLink(from, to) + (hash ? '#' + hash : ''));
	},
});
