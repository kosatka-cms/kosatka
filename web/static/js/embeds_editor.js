// Embeds: the text editor's "Insert…" button - a TinyMCE plugin the text
// formats with "Embeds" load (service.TextFilter.EditorPlugin) - and the
// cards over the tags. The button opens a window: the kinds on the left,
// the things of the chosen kind found by their title in the middle, what
// the chosen one shows on the right; "Insert" puts its tag
// ([[kind:name]]) in a paragraph of its own. A paragraph holding only a
// tag gets a card before it, the editor's own element
// (data-mce-bogus="all"): the saved text keeps just the tag. A card's
// "Change" opens the window on its thing, and the choice replaces the tag.
(function() {
    const api = '/api/embeds/';
    const tagRe = /^\[\[([a-z][a-z0-9_]*):([a-z0-9][a-z0-9_-]*)\]\]$/;

    // get - the JSON of an API address; null on a failure.
    function get(path) {
        return fetch(api + path, {credentials: 'same-origin'})
            .then(function(r) { return r.ok ? r.json() : null; })
            .catch(function() { return null; });
    }

    // kinds - the kinds the window offers ({name, label}), asked once a page.
    let kindsLoading = null;
    function kinds() {
        if (!kindsLoading) {
            kindsLoading = get('kinds').then(function(data) { return (data && data.kinds) || []; });
        }
        return kindsLoading;
    }
    function kindLabel(list, name) {
        const k = list.find(function(k) { return k.name === name; });
        return k ? k.label : name;
    }

    // describe - what each of tags ("kind:name") shows, asked once a page:
    // tag → its description, or null for nothing.
    const described = {};
    function describe(tags) {
        const ask = tags.filter(function(t) { return !(t in described); });
        if (!ask.length) {
            return Promise.resolve(described);
        }
        ask.forEach(function(t) { described[t] = undefined; });
        const query = ask.map(function(t) { return 'tag=' + encodeURIComponent(t); }).join('&');
        return get('describe?' + query).then(function(data) {
            const items = (data && data.items) || {};
            ask.forEach(function(t) { described[t] = items[t] || null; });
            return described;
        });
    }

    // paragraphTag - the tag ("kind:name") a paragraph holds and nothing
    // else.
    function paragraphTag(p) {
        const m = (p.textContent || '').trim().match(tagRe);
        return m ? m[1] + ':' + m[2] : '';
    }

    function el(doc, tag, cls, text) {
        const e = doc.createElement(tag);
        if (cls) {
            e.className = cls;
        }
        if (text) {
            e.textContent = text;
        }
        return e;
    }

    const cardStyle = '.kosatka-embed-card{display:flex;align-items:flex-start;gap:12px;margin:12px 0 4px;padding:10px 12px;' +
        'border:1px dashed #7fb3c4;border-radius:10px;background:#f3f9fb;color:#10202b;font:14px/1.4 system-ui,sans-serif;user-select:none}' +
        '.kosatka-embed-card.is-missing{border-color:#c98a1b;background:#fff7e6;color:#5a3a00}' +
        '.kosatka-embed-card .kosatka-embed-thumbs{flex:none;display:flex;gap:4px}' +
        '.kosatka-embed-card .kosatka-embed-thumbs img{width:52px;height:40px;object-fit:cover;border-radius:6px;background:#10202b}' +
        '.kosatka-embed-card .kosatka-embed-kind{flex:none;min-width:40px;height:40px;padding:0 6px;box-sizing:border-box;border-radius:8px;background:#dcecf2;color:#0b5c7a;display:flex;align-items:center;justify-content:center;font-size:12px;font-weight:700}' +
        '.kosatka-embed-card.is-missing .kosatka-embed-kind{background:#fde7bd;color:#6b4500}' +
        '.kosatka-embed-card strong{display:block}' +
        '.kosatka-embed-card .kosatka-embed-text{flex:1;min-width:0}' +
        '.kosatka-embed-card .kosatka-embed-text span{display:block;color:#4a6272;font-size:13px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}' +
        '.kosatka-embed-card.is-missing .kosatka-embed-text span{color:#6b4500}' +
        '.kosatka-embed-card button{flex:none;border:0;background:none;color:#0b5c7a;font:13px system-ui,sans-serif;cursor:pointer;padding:4px}' +
        '.kosatka-embed-card+p{margin-top:0;color:#4a6272;font-size:.85em;font-family:ui-monospace,Menlo,monospace}';

    // card - the card of tag (its description desc; null - nothing there).
    function card(doc, tag, desc, labels) {
        const kind = tag.split(':')[0];
        const c = el(doc, 'div', 'kosatka-embed-card' + (desc ? '' : ' is-missing'));
        c.dataset.tag = tag;
        c.setAttribute('data-mce-bogus', 'all');
        c.setAttribute('contenteditable', 'false');
        if (desc && desc.thumbs && desc.thumbs.length) {
            const thumbs = el(doc, 'div', 'kosatka-embed-thumbs');
            desc.thumbs.slice(0, 3).forEach(function(src) {
                const img = el(doc, 'img');
                img.src = src;
                img.alt = '';
                thumbs.appendChild(img);
            });
            c.appendChild(thumbs);
        } else {
            c.appendChild(el(doc, 'div', 'kosatka-embed-kind', desc ? kindLabel(labels, kind) : '!'));
        }
        const text = el(doc, 'div', 'kosatka-embed-text');
        if (desc) {
            text.appendChild(el(doc, 'strong', '', kindLabel(labels, kind) + ' · ' + desc.title));
            // What it shows (its text, items or count), then who sees it.
            const line = desc.lines && desc.lines.length ? desc.lines.join(' · ') : desc.meta || '';
            if (line) {
                text.appendChild(el(doc, 'span', '', line));
            }
            if (desc.note) {
                text.appendChild(el(doc, 'span', '', desc.note));
            }
        } else {
            text.appendChild(el(doc, 'strong', '', Kosatka.t('Nothing found for @tag', {'@tag': '[[' + tag + ']]'})));
            text.appendChild(el(doc, 'span', '', Kosatka.t('Nothing shows here on the site: it was deleted, renamed, or is not yours to see.')));
        }
        c.appendChild(text);
        const change = el(doc, 'button', '', desc ? Kosatka.t('Change') : Kosatka.t('Choose another'));
        change.type = 'button';
        change.setAttribute('data-embed-change', '');
        c.appendChild(change);
        return c;
    }

    // ===== The window =====

    // openWindow - the window choosing what to embed, on current ("kind:
    // name") when given; done(tag) gets the choice.
    function openWindow(current, done, onClose) {
        const win = Kosatka.window({
            title: Kosatka.t('Insert into the text'),
            className: 'kosatka-embed',
            onClose: onClose,
            onKey: onKey
        });
        const nav = el(document, 'nav', 'kosatka-embed-kinds');
        nav.setAttribute('aria-label', Kosatka.t('What to insert'));
        const listCol = el(document, 'div', 'kosatka-embed-list');
        const search = el(document, 'input');
        search.type = 'search';
        search.placeholder = Kosatka.t('Find by title');
        search.setAttribute('aria-label', Kosatka.t('Find by title'));
        const options = el(document, 'ul', 'kosatka-embed-options');
        options.setAttribute('role', 'listbox');
        options.setAttribute('aria-label', Kosatka.t('Found'));
        options.tabIndex = 0;
        listCol.append(search, options);
        const preview = el(document, 'div', 'kosatka-embed-preview');
        preview.setAttribute('aria-live', 'polite');
        win.body.append(nav, listCol, preview);
        const cancel = win.button(Kosatka.t('Cancel'));
        const insert = win.button(current ? Kosatka.t('Replace') : Kosatka.tc('verb', 'Insert'), true);
        insert.disabled = true;

        let kind = current ? current.split(':')[0] : '';
        let chosen = current || '';
        let found = [];
        let timer = 0;
        let asked = 0;

        function choose(tag) {
            chosen = tag;
            insert.disabled = !tag;
            options.querySelectorAll('li').forEach(function(li) {
                li.setAttribute('aria-selected', li.dataset.tag === tag ? 'true' : 'false');
            });
            const li = options.querySelector('li[aria-selected=true]');
            if (li) {
                li.scrollIntoView({block: 'nearest'});
            }
            showPreview(tag);
        }
        function showPreview(tag) {
            preview.textContent = '';
            if (!tag) {
                preview.appendChild(el(document, 'p', '', Kosatka.t('Choose what to insert: its preview shows here.')));
                return;
            }
            describe([tag]).then(function(all) {
                if (chosen !== tag) {
                    return;
                }
                preview.textContent = '';
                const d = all[tag];
                preview.appendChild(el(document, 'span', 'kosatka-embed-label', Kosatka.t('On the page')));
                if (!d) {
                    preview.appendChild(el(document, 'p', '', Kosatka.t('Nothing found for @tag', {'@tag': '[[' + tag + ']]'})));
                    return;
                }
                preview.appendChild(el(document, 'h3', '', d.title));
                if (d.meta) {
                    preview.appendChild(el(document, 'p', '', d.meta));
                }
                if (d.thumbs && d.thumbs.length) {
                    const grid = el(document, 'div', 'kosatka-embed-grid');
                    d.thumbs.forEach(function(src) {
                        const img = el(document, 'img');
                        img.src = src;
                        img.alt = '';
                        grid.appendChild(img);
                    });
                    preview.appendChild(grid);
                }
                if (d.lines && d.lines.length) {
                    const ul = el(document, 'ul');
                    d.lines.forEach(function(line) { ul.appendChild(el(document, 'li', '', line)); });
                    preview.appendChild(ul);
                }
                if (d.more) {
                    preview.appendChild(el(document, 'p', '', Kosatka.t('And @count more', {'@count': d.more})));
                }
                if (d.note) {
                    preview.appendChild(el(document, 'p', '', d.note));
                }
                const code = el(document, 'p', '', Kosatka.t('Tag:') + ' ');
                code.appendChild(el(document, 'code', '', '[[' + tag + ']]'));
                preview.appendChild(code);
            });
        }
        function load() {
            const ticket = ++asked;
            options.textContent = '';
            get('search?kind=' + encodeURIComponent(kind) + '&q=' + encodeURIComponent(search.value.trim())).then(function(data) {
                if (ticket !== asked) {
                    return;
                }
                found = (data && data.options) || [];
                options.textContent = '';
                if (!found.length) {
                    options.appendChild(el(document, 'li', 'kosatka-embed-empty', Kosatka.t('Nothing found')));
                }
                found.forEach(function(o) {
                    const li = el(document, 'li');
                    li.setAttribute('role', 'option');
                    li.dataset.tag = kind + ':' + o.name;
                    if (o.thumbs && o.thumbs.length) {
                        const img = el(document, 'img');
                        img.src = o.thumbs[0];
                        img.alt = '';
                        li.appendChild(img);
                    } else {
                        li.appendChild(el(document, 'div', 'kosatka-embed-nothumb'));
                    }
                    const text = el(document, 'div');
                    text.appendChild(el(document, 'strong', '', o.title));
                    if (o.meta) {
                        text.appendChild(el(document, 'span', '', o.meta));
                    }
                    li.appendChild(text);
                    options.appendChild(li);
                });
                choose(found.some(function(o) { return kind + ':' + o.name === chosen; }) ? chosen : '');
            });
        }
        function pickKind(name) {
            kind = name;
            nav.querySelectorAll('button').forEach(function(b) {
                b.setAttribute('aria-current', b.dataset.kind === name ? 'true' : 'false');
            });
            load();
        }
        // The list's keys (Escape is the window's).
        function onKey(e) {
            if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && (document.activeElement === search || document.activeElement === options)) {
                e.preventDefault();
                const tags = found.map(function(o) { return kind + ':' + o.name; });
                if (!tags.length) {
                    return;
                }
                const i = tags.indexOf(chosen);
                choose(tags[e.key === 'ArrowDown' ? Math.min(i + 1, tags.length - 1) : Math.max(i - 1, 0)]);
                return;
            }
            if (e.key === 'Enter' && chosen && (document.activeElement === search || document.activeElement === options)) {
                e.preventDefault();
                insert.click();
            }
        }

        options.addEventListener('click', function(e) {
            const li = e.target.closest('li[data-tag]');
            if (li) {
                choose(li.dataset.tag);
            }
        });
        options.addEventListener('dblclick', function(e) {
            if (e.target.closest('li[data-tag]')) {
                insert.click();
            }
        });
        search.addEventListener('input', function() {
            clearTimeout(timer);
            timer = setTimeout(load, 250);
        });
        nav.addEventListener('click', function(e) {
            const b = e.target.closest('button[data-kind]');
            if (b) {
                pickKind(b.dataset.kind);
                search.focus();
            }
        });
        cancel.addEventListener('click', win.close);
        insert.addEventListener('click', function() {
            if (!chosen) {
                return;
            }
            const tag = chosen;
            win.close();
            done(tag);
        });

        showPreview('');
        kinds().then(function(list) {
            if (!list.length) {
                options.appendChild(el(document, 'li', 'kosatka-embed-empty', Kosatka.t('Nothing to insert')));
                return;
            }
            list.forEach(function(k) {
                const b = el(document, 'button', '', k.label);
                b.type = 'button';
                b.dataset.kind = k.name;
                nav.appendChild(b);
            });
            if (!list.some(function(k) { return k.name === kind; })) {
                kind = list[0].name;
            }
            pickKind(kind);
            search.focus();
        });
    }

    tinymce.PluginManager.add('kosatkaembed', function(editor) {
        // refresh - a card before each paragraph holding only a tag, none
        // elsewhere.
        function refresh() {
            const body = editor.getBody();
            if (!body) {
                return;
            }
            body.querySelectorAll('.kosatka-embed-card').forEach(function(c) {
                const p = c.nextElementSibling;
                if (!p || p.tagName !== 'P' || paragraphTag(p) !== c.dataset.tag) {
                    c.remove();
                }
            });
            const paragraphs = [];
            body.querySelectorAll('p').forEach(function(p) {
                const tag = paragraphTag(p);
                const before = p.previousElementSibling;
                if (tag && !(before && before.classList.contains('kosatka-embed-card') && before.dataset.tag === tag)) {
                    paragraphs.push({p: p, tag: tag});
                }
            });
            if (!paragraphs.length) {
                return;
            }
            Promise.all([kinds(), describe(paragraphs.map(function(x) { return x.tag; }))]).then(function(res) {
                paragraphs.forEach(function(x) {
                    if (!x.p.isConnected || paragraphTag(x.p) !== x.tag) {
                        return;
                    }
                    const prev = x.p.previousElementSibling;
                    if (prev && prev.classList.contains('kosatka-embed-card')) {
                        return;
                    }
                    x.p.before(card(editor.getDoc(), x.tag, res[1][x.tag], res[0]));
                });
            });
        }
        let timer = 0;
        function refreshSoon() {
            clearTimeout(timer);
            timer = setTimeout(refresh, 300);
        }
        // put - tag in a paragraph of its own at the cursor, the cursor in an
        // empty paragraph after it; or in place of the paragraph p.
        function put(tag, p) {
            if (p && p.isConnected) {
                editor.undoManager.transact(function() {
                    p.textContent = '[[' + tag + ']]';
                });
                editor.nodeChanged();
            } else {
                editor.insertContent('<p>' + editor.dom.encode('[[' + tag + ']]') + '</p><p data-kosatka-after="1"><br data-mce-bogus="1"></p>');
                const after = editor.dom.select('p[data-kosatka-after]')[0];
                if (after) {
                    after.removeAttribute('data-kosatka-after');
                    editor.selection.setCursorLocation(after, 0);
                }
            }
            refreshSoon();
        }
        editor.on('init', function() {
            editor.dom.addStyle(cardStyle);
            refresh();
            editor.getBody().addEventListener('click', function(e) {
                const b = e.target.closest('[data-embed-change]');
                if (!b) {
                    return;
                }
                e.preventDefault();
                const c = b.closest('.kosatka-embed-card');
                const p = c && c.nextElementSibling;
                openWindow(c.dataset.tag, function(tag) { put(tag, p); }, function() { editor.focus(); });
            });
        });
        editor.on('SetContent Undo Redo Change paste', refreshSoon);
        editor.on('keyup', function(e) {
            if (e.key === 'Enter' || e.key === ']') {
                refreshSoon();
            }
        });
        editor.ui.registry.addButton('kosatkaembed', {
            icon: 'plus',
            text: Kosatka.t('Insert…'),
            tooltip: Kosatka.t('Insert a block, a list, a gallery, a poll or a form'),
            onAction: function() {
                openWindow('', function(tag) { put(tag); }, function() { editor.focus(); });
            }
        });
        return {getMetadata: function() { return {name: 'Kosatka embeds'}; }};
    });
})();
