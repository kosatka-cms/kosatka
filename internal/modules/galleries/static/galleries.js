// Galleries module (internal/modules/galleries), loaded after app.js
// (attached by its templates, {{attach}}).

// ===== Lazy loading of the galleries block on a content page =====
// nodeGalleries - the block (.node-galleries-lazy-mount, the
// block_node_galleries plugin block) loads its galleries
// once it nears the viewport; the behaviors run for them (the images open
// in the content lightbox, app.js's contentGallery).
Kosatka.behaviors.nodeGalleries = {
    attach: function (context) {
        Kosatka.once('node-galleries', '.node-galleries-lazy-mount', context).forEach(initNodeGalleriesLazy);
    }
};

function initNodeGalleriesLazy(mount) {
    const inner = mount.querySelector('.node-galleries-inner');
    const sentinel = mount.querySelector('.node-galleries-sentinel');
    const endpoint = mount.getAttribute('data-endpoint');
    if (!endpoint || !inner) {
        return;
    }

    const runLoad = async () => {
        if (mount.getAttribute('data-loaded') === '1') {
            return;
        }
        mount.setAttribute('data-loaded', '1');
        try {
            const res = await fetch(endpoint, { credentials: 'same-origin' });
            if (!res.ok) {
                mount.remove();
                return;
            }
            const html = (await res.text()).trim();
            if (html) {
                inner.innerHTML = html;
                mount.classList.add('node-galleries-loaded');
                Kosatka.attachBehaviors(inner);
            } else {
                mount.remove();
            }
        } catch (e) {
            console.error('node galleries lazy load', e);
            mount.remove();
        }
    };

    if (!('IntersectionObserver' in window)) {
        runLoad();
        return;
    }

    const target = sentinel || mount;
    const observer = new IntersectionObserver((entries) => {
        for (let i = 0; i < entries.length; i++) {
            if (entries[i].isIntersecting) {
                observer.disconnect();
                runLoad();
                break;
            }
        }
    }, { rootMargin: '240px', threshold: 0.01 });

    observer.observe(target);
}
