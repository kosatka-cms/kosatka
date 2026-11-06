// The pages' "Edit page" link: Starlight appends the page's path in the
// site's project (src/content/docs/...), but src/content/docs is a link to
// the repository's docs/, where the file is: docs/<path>.
import { defineRouteMiddleware } from '@astrojs/starlight/route-data';

export const onRequest = defineRouteMiddleware((context) => {
	const route = context.locals.starlightRoute;
	if (route.editUrl) {
		route.editUrl = new URL(route.editUrl.href.replace('/src/content/docs/', '/'));
	}
});
