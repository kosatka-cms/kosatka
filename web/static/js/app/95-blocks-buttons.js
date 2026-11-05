// lazyBlocks - lazy block placements (BlockPlacementDef.Lazy,
// partials/region_blocks.html): each [data-lazy-block] holds the
// theme's placeholder and the URL of the block's content; the content
// replaces the placeholder once the place scrolls within 400px of the
// viewport (right away without IntersectionObserver), and the behaviors
// run for it.
Kosatka.behaviors.lazyBlocks = {
    attach: function (context) {
        var places = Kosatka.once('lazy-block', '[data-lazy-block]', context);
        if (!places.length) return;
        function load(el) {
            if (el.getAttribute('data-lazy-state')) return;
            el.setAttribute('data-lazy-state', 'loading');
            fetch(el.getAttribute('data-lazy-block'), { credentials: 'same-origin' })
                .then(function (r) {
                    if (!r.ok) return Promise.reject(new Error('HTTP ' + r.status));
                    return r.text();
                })
                .then(function (html) {
                    el.innerHTML = html;
                    el.setAttribute('data-lazy-state', 'loaded');
                    Kosatka.attachBehaviors(el);
                })
                .catch(function () { el.setAttribute('data-lazy-state', 'error'); });
        }
        if (!('IntersectionObserver' in window)) {
            places.forEach(load);
            return;
        }
        var io = new IntersectionObserver(function (entries) {
            entries.forEach(function (en) {
                if (en.isIntersecting) {
                    io.unobserve(en.target);
                    load(en.target);
                }
            });
        }, { root: null, rootMargin: '400px 0px', threshold: 0 });
        places.forEach(function (el) { io.observe(el); });
    }
};

// copyText - a button copying its data-copy-text to the clipboard (its
// data-copied - the words it shows a moment after); print-page - a
// button printing the page ([data-print]).
Kosatka.behaviors.copyText = {
    attach: function (context) {
        Kosatka.once('copy-text', '[data-copy-text]', context).forEach(function (button) {
            button.addEventListener('click', function () {
                if (!navigator.clipboard) return;
                navigator.clipboard.writeText(button.getAttribute('data-copy-text')).then(function () {
                    var words = button.textContent;
                    button.textContent = button.getAttribute('data-copied') || words;
                    setTimeout(function () { button.textContent = words; }, 1500);
                });
            });
        });
        Kosatka.once('print-page', '[data-print]', context).forEach(function (button) {
            button.addEventListener('click', function () { window.print(); });
        });
    }
};

// passwordToggle - a button showing and hiding the password of the field
// its data-password-toggle names (aria-pressed; its data-label-hide -
// its label while the password shows).
Kosatka.behaviors.passwordToggle = {
    attach: function (context) {
        Kosatka.once('password-toggle', '[data-password-toggle]', context).forEach(function (button) {
            var input = document.getElementById(button.getAttribute('data-password-toggle'));
            if (!input) return;
            var showLabel = button.getAttribute('aria-label');
            button.addEventListener('click', function () {
                var show = input.type === 'password';
                input.type = show ? 'text' : 'password';
                button.setAttribute('aria-pressed', show ? 'true' : 'false');
                button.setAttribute('aria-label', show ? (button.getAttribute('data-label-hide') || showLabel) : showLabel);
                input.focus();
            });
        });
    }
};
