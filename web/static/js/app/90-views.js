// initViewCarousels - Swiper init for every [data-view-carousel] container
// (Views DisplayStyle "carousel"), with options from its data-* attributes
// (set by dynamic/list.html from entity.ViewDisplayDef.Carousel*; a theme's
// View template may set them too, and two more of its own:
// data-space-between - the gap in px at every width, data-responsive="0" -
// data-slides-per-view at every width instead of fewer on narrow screens). Run
// by the viewCarousels behavior: on page load and for every piece of HTML
// put in the page (an AJAX page of a list, a lazy block). Existing
// instances are destroyed and recreated: Swiper can't cheaply re-read its
// options.
function initViewCarousels(root) {
    if (typeof Swiper === 'undefined') return;
    var scope = root || document;
    var containers = scope.querySelectorAll('[data-view-carousel]');
    containers.forEach(function (container) {
        if (container.swiper) {
            container.swiper.destroy(true, true);
        }
        var slidesPerView = parseInt(container.getAttribute('data-slides-per-view'), 10) || 1;
        // data-slides-per-view is the desktop count; narrower screens show fewer
        // slides (capped by it) so cards don't get squeezed on phones.
        var options = {
            slidesPerView: Math.min(slidesPerView, 2),
            spaceBetween: 10,
            watchOverflow: true,
            loop: container.getAttribute('data-loop') === '1',
            breakpoints: {
                640: { slidesPerView: Math.min(slidesPerView, 3), spaceBetween: 15 },
                1024: { slidesPerView: slidesPerView, spaceBetween: 20 }
            }
        };
        if (container.getAttribute('data-responsive') === '0') {
            options.slidesPerView = slidesPerView;
            delete options.breakpoints;
        }
        var spaceBetween = container.getAttribute('data-space-between');
        if (spaceBetween !== null && spaceBetween !== '') {
            options.spaceBetween = parseInt(spaceBetween, 10) || 0;
            if (options.breakpoints) {
                options.breakpoints[640].spaceBetween = options.spaceBetween;
                options.breakpoints[1024].spaceBetween = options.spaceBetween;
            }
        }
        // effect - Swiper's transition style (entity.ViewDisplayDef.CarouselEffect);
        // "fade" crossfades instead of sliding.
        if (container.getAttribute('data-effect') === 'fade') {
            options.effect = 'fade';
            options.fadeEffect = { crossFade: true };
        }
        if (container.getAttribute('data-autoplay') === '1') {
            options.autoplay = { delay: 5000, disableOnInteraction: false };
        }
        if (container.getAttribute('data-show-navigation') === '1') {
            options.navigation = {
                nextEl: container.querySelector('.swiper-button-next'),
                prevEl: container.querySelector('.swiper-button-prev')
            };
        }
        if (container.getAttribute('data-show-pagination') === '1') {
            options.pagination = {
                el: container.querySelector('.swiper-pagination'),
                clickable: true
            };
        }
        try {
            new Swiper(container, options);
        } catch (error) {
            console.error('Error initializing view carousel:', error);
        }
    });
}

Kosatka.behaviors.viewCarousels = {
    attach: function (context) {
        initViewCarousels(context);
    }
};

// initViewPagination - AJAX paging for Views: a View page that opts in
// (entity.ViewDisplayDef.PaginationAjax) and a paged View block mark their list
// container data-ajax-pagination="1"; a click on a pager link inside it
// re-fetches the href with X-Requested-With and swaps the fragment in. A
// page's links stay real "?page=N" hrefs (without JS - plain links); a
// block's lead to /api/view-block/{id} (without JS - back to the page). Delegated on the document: lazy
// blocks and swapped-in lists need no setup.
function initViewPagination() {
    document.addEventListener('click', function(e) {
        // Scope strictly to pager links - item cards/fields inside the
        // same container have their own real <a> links (title, image,
        // taxonomy term) that must keep navigating normally.
        const link = e.target.closest('.pagination a');
        const container = link ? link.closest('[data-ajax-pagination="1"]') : null;
        if (!container) return;
        e.preventDefault();
        loadViewPage(container, link.href);
    });
}

async function loadViewPage(container, url) {
    container.style.opacity = '0.5';
    container.style.pointerEvents = 'none';
    try {
        const response = await fetch(url, { headers: { 'X-Requested-With': 'XMLHttpRequest' } });
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        container.innerHTML = data.html;
        Kosatka.attachBehaviors(container);
        // A page's list scrolls to its top; a block only into view.
        container.scrollIntoView({ behavior: 'smooth', block: container.closest('.block') ? 'nearest' : 'start' });
    } catch (error) {
        console.error('Error loading view page:', error);
        // A page's list falls back to a real navigation (its links are
        // its own pages); a block's page has no page of its own: it stays.
        if (!new URL(url, window.location.href).pathname.startsWith('/api/view-block/')) {
            window.location.href = url;
        }
        return;
    } finally {
        container.style.opacity = '1';
        container.style.pointerEvents = 'auto';
    }
}

document.addEventListener('DOMContentLoaded', initViewPagination);

// ===== Views: AJAX glossary (entity.ViewDisplayDef.ShowGlossary) =====
// Reuses loadViewPage's #view-list-container swap. The glossary bar
// (#view-glossary) is a sibling of that container, so a swap never wipes
// it; its .active class is updated here (the server sets it only on a full
// page load). Gated on the same data-ajax-pagination flag as pagination.
function initViewGlossary() {
    const glossary = document.getElementById('view-glossary');
    const container = document.getElementById('view-list-container');
    if (!glossary || !container || glossary.getAttribute('data-ajax-pagination') !== '1') return;

    glossary.addEventListener('click', function(e) {
        const link = e.target.closest('a.glossary-letter');
        if (!link) return;
        e.preventDefault();
        glossary.querySelectorAll('.glossary-letter.active').forEach(function(el) { el.classList.remove('active'); });
        link.classList.add('active');
        loadViewPage(container, link.href);
    });
}

document.addEventListener('DOMContentLoaded', initViewGlossary);

