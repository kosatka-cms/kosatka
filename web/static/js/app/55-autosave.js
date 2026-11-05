// ===== Autosave =====
// A content form (form[data-autosave="<key>"]) keeps what is typed in it
// in the browser (localStorage) every few seconds and on every change,
// so a tab closed, a browser crashed or a session ended loses nothing:
// opened again, the form offers the draft kept after its last save
// (data-autosave-since - the content's update time, seconds) to restore
// or discard. The draft is dropped once the form is saved: the server
// names its key in the kosatka_draft_saved cookie, read on the next page -
// a submit refused (an expired session, a stale token) keeps it. The
// fields the form has are restored (text, choices, the text editors'
// HTML); file inputs and hidden ones aren't kept. Per browser and per
// user (the key holds the user's ID).
(function () {
    const prefix = 'kosatka-autosave:';
    const savedCookie = 'kosatka_draft_saved';
    const every = 10000;

    function store() {
        try {
            return window.localStorage;
        } catch (e) {
            return null;
        }
    }

    // A saved form's draft goes, whatever the page (before a form here
    // could offer it); the cookie with it.
    const saved = document.cookie.match(new RegExp('(?:^|; )' + savedCookie + '=([^;]*)'));
    if (saved) {
        document.cookie = savedCookie + '=; Max-Age=0; path=/; SameSite=Lax' + (location.protocol === 'https:' ? '; Secure' : '');
        const ls = store();
        if (ls) {
            try {
                ls.removeItem(prefix + decodeURIComponent(saved[1]));
            } catch (e) { /* blocked: nothing to drop */ }
        }
    }
    function kept(el) {
        if (!el.name || el.disabled || el.name === 'csrf_token') {
            return false;
        }
        const type = (el.type || '').toLowerCase();
        return type !== 'file' && type !== 'hidden' && type !== 'password' && type !== 'submit' && type !== 'button';
    }
    // values - the form's kept fields as [name, value] pairs (a checkbox
    // or radio only when checked; each selected option of a select).
    function values(form) {
        if (window.tinymce) {
            window.tinymce.triggerSave();
        }
        const out = [];
        Array.prototype.forEach.call(form.elements, function (el) {
            if (!kept(el)) {
                return;
            }
            const type = el.type.toLowerCase();
            if ((type === 'checkbox' || type === 'radio') && !el.checked) {
                return;
            }
            if (el.tagName === 'SELECT' && el.multiple) {
                Array.prototype.forEach.call(el.selectedOptions, function (o) { out.push([el.name, o.value]); });
                return;
            }
            out.push([el.name, el.value]);
        });
        return out;
    }
    function restore(form, pairs) {
        const byName = {};
        pairs.forEach(function (p) { (byName[p[0]] = byName[p[0]] || []).push(p[1]); });
        Array.prototype.forEach.call(form.elements, function (el) {
            if (!kept(el) || !(el.name in byName)) {
                return;
            }
            const vals = byName[el.name];
            const type = el.type.toLowerCase();
            if (type === 'checkbox' || type === 'radio') {
                el.checked = vals.indexOf(el.value) !== -1;
            } else if (el.tagName === 'SELECT') {
                Array.prototype.forEach.call(el.options, function (o) { o.selected = vals.indexOf(o.value) !== -1; });
            } else {
                el.value = vals.shift();
                byName[el.name] = vals;
                const editor = window.tinymce && el.id ? window.tinymce.get(el.id) : null;
                if (editor) {
                    editor.setContent(el.value);
                }
            }
            el.dispatchEvent(new Event('change', {bubbles: true}));
        });
    }
    function same(a, b) {
        return JSON.stringify(a) === JSON.stringify(b);
    }

    Kosatka.behaviors.autosave = {
        attach: function (context) {
            const ls = store();
            if (!ls) {
                return;
            }
            Kosatka.once('autosave', 'form[data-autosave]', context).forEach(function (form) {
                const key = prefix + form.getAttribute('data-autosave');
                const since = (parseInt(form.getAttribute('data-autosave-since'), 10) || 0) * 1000;
                const initial = values(form);
                let draft = null;
                try {
                    draft = JSON.parse(ls.getItem(key) || 'null');
                } catch (e) {
                    draft = null;
                }
                if (draft && draft.at > since && !same(draft.values, initial)) {
                    const note = document.createElement('div');
                    note.className = 'alert alert-info node-autosave';
                    note.setAttribute('role', 'status');
                    const text = document.createElement('span');
                    text.textContent = Kosatka.t('Unsaved changes from @time are kept in this browser.', {'@time': new Date(draft.at).toLocaleString(document.documentElement.lang || undefined)}) + ' ';
                    const restoreBtn = document.createElement('button');
                    restoreBtn.type = 'button';
                    restoreBtn.className = 'btn btn-sm btn-primary';
                    restoreBtn.textContent = Kosatka.t('Restore them');
                    const dropBtn = document.createElement('button');
                    dropBtn.type = 'button';
                    dropBtn.className = 'btn btn-sm btn-outline-secondary ms-2';
                    dropBtn.textContent = Kosatka.t('Discard');
                    note.append(text, restoreBtn, dropBtn);
                    form.before(note);
                    restoreBtn.addEventListener('click', function () {
                        restore(form, draft.values);
                        note.remove();
                    });
                    dropBtn.addEventListener('click', function () {
                        ls.removeItem(key);
                        note.remove();
                    });
                } else if (draft) {
                    ls.removeItem(key);
                }
                let last = JSON.stringify(initial);
                function save() {
                    const now = values(form);
                    const json = JSON.stringify(now);
                    if (json === last) {
                        return;
                    }
                    last = json;
                    try {
                        ls.setItem(key, JSON.stringify({at: Date.now(), values: now}));
                    } catch (e) { /* full or blocked: nothing kept */ }
                }
                let timer = 0;
                form.addEventListener('input', function () {
                    clearTimeout(timer);
                    timer = setTimeout(save, 1500);
                });
                form.addEventListener('change', function () {
                    clearTimeout(timer);
                    timer = setTimeout(save, 1500);
                });
                // The text editors type into their own frames: a regular save.
                setInterval(save, every);
                // Sent: the draft holds what was sent, until the server says
                // it's saved.
                form.addEventListener('submit', save);
            });
        }
    };
})();
