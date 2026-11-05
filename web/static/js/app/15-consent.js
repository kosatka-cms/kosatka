// ===== Cookie consent =====
// The banner ([data-consent-banner], partials/html.html consent_banner)
// and the codes waiting for consent (<template data-consent="<group>">):
// the visitor's choice is the kosatka_consent cookie, "v<version>.<the
// allowed groups, + between>" ("v3.analytics", "v3." - the necessary
// only; notice mode - "v3.ok"), a year. A choice of this version runs the
// allowed groups' codes and keeps the banner hidden; none - the banner
// shows. A link to #cookie-settings opens it again; taking a group back
// reloads the page (its code already ran).
(function () {
    var banner = document.querySelector('[data-consent-banner]');
    var cookieName = 'kosatka_consent';

    function stored() {
        var m = document.cookie.match(/(?:^|;\s*)kosatka_consent=([^;]*)/);
        if (!m || !banner) return null;
        var value = decodeURIComponent(m[1]);
        var prefix = 'v' + banner.getAttribute('data-consent-version') + '.';
        if (value.indexOf(prefix) !== 0) return null;
        var rest = value.slice(prefix.length);
        return rest === '' ? [] : rest.split('+');
    }
    function store(groups) {
        var value = 'v' + banner.getAttribute('data-consent-version') + '.' + groups.join('+');
        document.cookie = cookieName + '=' + encodeURIComponent(value) + '; path=/; max-age=31536000; SameSite=Lax' +
            (location.protocol === 'https:' ? '; Secure' : '');
    }
    // run - the waiting codes of the allowed groups, each once: their
    // <script> elements made anew so that the browser runs them.
    function run(groups) {
        document.querySelectorAll('template[data-consent]').forEach(function (t) {
            if (t.hasAttribute('data-consent-ran') || groups.indexOf(t.getAttribute('data-consent')) === -1) return;
            t.setAttribute('data-consent-ran', '');
            var code = t.content.cloneNode(true);
            code.querySelectorAll('script').forEach(function (old) {
                var s = document.createElement('script');
                Array.prototype.forEach.call(old.attributes, function (a) { s.setAttribute(a.name, a.value); });
                s.text = old.text;
                old.replaceWith(s);
            });
            t.parentNode.insertBefore(code, t);
        });
    }

    var choice = stored();
    if (choice) run(choice);
    if (!banner) return;

    var groupsBox = banner.querySelector('[data-consent-groups]');
    var saveButton = banner.querySelector('[data-consent-save]');
    var allButton = banner.querySelector('[data-consent-all]');
    var customize = banner.querySelector('[data-consent-customize]');
    function ticks() {
        return Array.prototype.slice.call(banner.querySelectorAll('[data-consent-group]'));
    }
    function show(open) {
        banner.hidden = false;
        if (open && groupsBox) {
            groupsBox.hidden = false;
            if (saveButton) saveButton.hidden = false;
            if (allButton) allButton.hidden = true;
            if (customize) customize.hidden = true;
            var current = stored() || [];
            ticks().forEach(function (box) { box.checked = current.indexOf(box.value) !== -1; });
        }
    }
    function decide(groups) {
        var before = stored();
        store(groups);
        banner.hidden = true;
        if (before && before.some(function (g) { return groups.indexOf(g) === -1; })) {
            location.reload();
            return;
        }
        run(groups);
    }

    banner.addEventListener('click', function (e) {
        var b = e.target.closest('button');
        if (!b) return;
        if (b.hasAttribute('data-consent-ok')) {
            decide(['ok']);
        } else if (b.hasAttribute('data-consent-all')) {
            decide(ticks().map(function (box) { return box.value; }));
        } else if (b.hasAttribute('data-consent-necessary')) {
            decide([]);
        } else if (b.hasAttribute('data-consent-customize')) {
            show(true);
        } else if (b.hasAttribute('data-consent-save')) {
            decide(ticks().filter(function (box) { return box.checked; }).map(function (box) { return box.value; }));
        }
    });
    document.addEventListener('click', function (e) {
        var link = e.target.closest('a[href="#cookie-settings"]');
        if (!link) return;
        e.preventDefault();
        show(banner.getAttribute('data-consent-mode') === 'consent');
    });
    if (!choice) show(false);
})();
