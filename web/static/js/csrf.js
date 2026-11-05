// CSRF token for forms and scripts (partials/html.html, csrf_head, loads
// this right after the csrf-token meta tag, before any other script):
// every POST form gets a csrf_token field (on load, when added later and on
// submit), and fetch/XMLHttpRequest send it in X-CSRF-Token.
// middleware.CSRFMiddleware rejects state-changing requests without it. A
// guest who only reads has no token (no session, so the page can be
// cached): the first POST form or request asks /api/csrf-token for one,
// then goes on.
(function () {
    var meta = document.querySelector('meta[name="csrf-token"]');
    var token = meta.content;
    var origFetch = window.fetch;
    var pending = null;
    // getToken - the token, asked for once when the page has none.
    function getToken() {
        if (token) return Promise.resolve(token);
        if (!pending) {
            pending = origFetch.call(window, '/api/csrf-token', {credentials: 'same-origin', cache: 'no-store'})
                .then(function (r) { return r.json(); })
                .then(function (d) {
                    token = (d && d.token) || '';
                    meta.content = token;
                    addAll(document);
                    return token;
                });
        }
        return pending;
    }
    function isPostForm(form) {
        return form && (form.method || '').toLowerCase() === 'post';
    }
    function addField(form) {
        if (!token || !isPostForm(form)) return;
        var input = form.querySelector('input[name="csrf_token"]');
        if (input) {
            if (!input.value) input.value = token;
            return;
        }
        input = document.createElement('input');
        input.type = 'hidden';
        input.name = 'csrf_token';
        input.value = token;
        form.appendChild(input);
    }
    function addAll(root) {
        if (root.tagName === 'FORM') addField(root);
        if (root.querySelectorAll) root.querySelectorAll('form').forEach(addField);
    }
    document.addEventListener('submit', function (e) {
        var form = e.target;
        if (!isPostForm(form)) return;
        if (token) {
            addField(form);
            return;
        }
        // No token yet: hold the submit, get one, submit again.
        e.preventDefault();
        e.stopImmediatePropagation();
        var submitter = e.submitter;
        getToken().then(function () {
            addField(form);
            if (form.requestSubmit) form.requestSubmit(submitter && submitter.form === form ? submitter : undefined);
            else form.submit();
        });
    }, true);
    document.addEventListener('DOMContentLoaded', function () {
        addAll(document);
        new MutationObserver(function (records) {
            records.forEach(function (r) { r.addedNodes.forEach(function (n) { if (n.nodeType === 1) addAll(n); }); });
        }).observe(document.body, { childList: true, subtree: true });
    });
    var unsafe = function (m) { return !/^(GET|HEAD|OPTIONS)$/i.test(m || 'GET'); };
    // exempt - POST endpoints taking no token (middleware.csrfExemptPaths).
    var exempt = ['/api/node/view'];
    function sameOriginPath(url) {
        url = String(url);
        if (url.indexOf('//') !== -1 && url.indexOf(location.origin + '/') !== 0) return null;
        return new URL(url, location.href).pathname;
    }
    if (origFetch) {
        window.fetch = function (input, init) {
            init = init || {};
            var method = init.method || (input && input.method) || 'GET';
            var path = sameOriginPath(typeof input === 'string' ? input : (input && input.url) || '');
            var self = this;
            if (unsafe(method) && path !== null && exempt.indexOf(path) < 0) {
                var headers = new Headers(init.headers || (input && input.headers) || {});
                if (!headers.has('X-CSRF-Token')) {
                    return getToken().then(function (t) {
                        headers.set('X-CSRF-Token', t);
                        init.headers = headers;
                        return origFetch.call(self, input, init);
                    });
                }
                init.headers = headers;
            }
            return origFetch.call(this, input, init);
        };
    }
    var open = XMLHttpRequest.prototype.open, send = XMLHttpRequest.prototype.send;
    XMLHttpRequest.prototype.open = function (method, url) {
        this._csrfMethod = method;
        var path = sameOriginPath(url);
        this._csrfSameOrigin = path !== null && exempt.indexOf(path) < 0;
        return open.apply(this, arguments);
    };
    XMLHttpRequest.prototype.send = function () {
        if (!unsafe(this._csrfMethod) || !this._csrfSameOrigin) return send.apply(this, arguments);
        var xhr = this, args = arguments;
        if (token) {
            xhr.setRequestHeader('X-CSRF-Token', token);
            return send.apply(xhr, args);
        }
        getToken().then(function (t) {
            xhr.setRequestHeader('X-CSRF-Token', t);
            send.apply(xhr, args);
        });
    };
})();
