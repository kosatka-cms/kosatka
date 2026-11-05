// Video embed module: the text editor's "Video" button - a TinyMCE
// plugin the text formats with "Video links as players" load
// (modulekit.TextFilter.EditorPlugin) - and the cards a video link
// standing alone in a paragraph shows as in the editor. A card is the
// editor's own element (data-mce-bogus="all"): the saved text keeps just
// the link, the site shows the video in its place.
(function() {
    const api = '/api/video-embed?url=';
    const answers = {};

    // resolve - what the site says about the link (asked once a page).
    function resolve(url) {
        if (!answers[url]) {
            answers[url] = fetch(api + encodeURIComponent(url), {credentials: 'same-origin'})
                .then(function(r) { return r.ok ? r.json() : {ok: false}; })
                .catch(function() { return {ok: false}; });
        }
        return answers[url];
    }

    // standaloneLink - the link a paragraph holds and nothing else.
    function standaloneLink(p) {
        const text = (p.textContent || '').trim();
        if (!/^https?:\/\/\S+$/.test(text)) {
            return '';
        }
        const links = p.querySelectorAll('a');
        if (links.length > 1) {
            return '';
        }
        return links.length === 1 ? (links[0].getAttribute('href') || '').trim() : text;
    }

    const cardStyle = '.kosatka-video-card{display:flex;align-items:center;gap:12px;margin:12px 0 4px;padding:10px 12px;' +
        'border:1px dashed #7fb3c4;border-radius:10px;background:#f3f9fb;color:#10202b;font:14px/1.4 system-ui,sans-serif;user-select:none}' +
        '.kosatka-video-card img,.kosatka-video-card .kosatka-video-thumb{flex:none;width:96px;aspect-ratio:16/9;object-fit:cover;border-radius:6px;background:#10202b}' +
        '.kosatka-video-card strong{display:block}' +
        '.kosatka-video-card span{display:block;color:#4a6272;font-size:12px}' +
        '.kosatka-video-card+p{margin-top:0;color:#4a6272;font-size:.85em}';

    tinymce.PluginManager.add('kosatkavideo', function(editor) {
        // refresh - a card before each video link's paragraph, none
        // elsewhere.
        function refresh() {
            const body = editor.getBody();
            if (!body) {
                return;
            }
            body.querySelectorAll('.kosatka-video-card').forEach(function(card) {
                const p = card.nextElementSibling;
                if (!p || p.tagName !== 'P' || standaloneLink(p) !== card.dataset.url) {
                    card.remove();
                }
            });
            body.querySelectorAll('p').forEach(function(p) {
                const url = standaloneLink(p);
                if (!url) {
                    return;
                }
                const before = p.previousElementSibling;
                if (before && before.classList.contains('kosatka-video-card') && before.dataset.url === url) {
                    return;
                }
                resolve(url).then(function(info) {
                    if (!info.ok || !p.isConnected || standaloneLink(p) !== url) {
                        return;
                    }
                    const prev = p.previousElementSibling;
                    if (prev && prev.classList.contains('kosatka-video-card')) {
                        return;
                    }
                    const doc = editor.getDoc();
                    const card = doc.createElement('div');
                    card.className = 'kosatka-video-card';
                    card.dataset.url = url;
                    card.setAttribute('data-mce-bogus', 'all');
                    card.setAttribute('contenteditable', 'false');
                    let thumb;
                    if (info.preview) {
                        thumb = doc.createElement('img');
                        thumb.src = info.preview;
                        thumb.alt = '';
                    } else {
                        thumb = doc.createElement('div');
                        thumb.className = 'kosatka-video-thumb';
                    }
                    const text = doc.createElement('div');
                    const title = doc.createElement('strong');
                    title.textContent = info.title ? info.provider + ' · ' + info.title : info.provider;
                    const hint = doc.createElement('span');
                    hint.textContent = Kosatka.t('The link below shows as the video on the site.');
                    text.append(title, hint);
                    card.append(thumb, text);
                    p.before(card);
                });
            });
        }

        let timer = 0;
        function refreshSoon() {
            clearTimeout(timer);
            timer = setTimeout(refresh, 300);
        }

        editor.on('init', function() {
            editor.dom.addStyle(cardStyle);
            refresh();
        });
        editor.on('SetContent Undo Redo Change paste', refreshSoon);
        editor.on('keyup', function(e) {
            if (e.key === 'Enter' || e.key === ' ') {
                refreshSoon();
            }
        });

        editor.ui.registry.addButton('kosatkavideo', {
            icon: 'embed',
            text: Kosatka.t('Video'),
            tooltip: Kosatka.t('Insert a video by its link'),
            onAction: function() {
                editor.windowManager.open({
                    title: Kosatka.t('Insert a video'),
                    body: {
                        type: 'panel',
                        items: [
                            {type: 'input', name: 'url', label: Kosatka.t('Link to the video')},
                            {type: 'htmlpanel', html: '<p>' + editor.dom.encode(Kosatka.t('A link to a video of one of the services on the "Video embeds" page. The site shows the video in its place.')) + '</p>'}
                        ]
                    },
                    buttons: [
                        {type: 'cancel', text: Kosatka.t('Cancel')},
                        {type: 'submit', text: Kosatka.t('Add the video'), primary: true}
                    ],
                    onSubmit: function(dialog) {
                        const url = (dialog.getData().url || '').trim();
                        dialog.block(Kosatka.t('Checking the link…'));
                        resolve(url).then(function(info) {
                            dialog.unblock();
                            if (!info.ok) {
                                editor.notificationManager.open({text: Kosatka.t('Not a link to a video of the services on.'), type: 'warning', timeout: 4000});
                                return;
                            }
                            dialog.close();
                            editor.insertContent('<p>' + editor.dom.encode(url) + '</p>');
                            refreshSoon();
                        });
                    }
                });
            }
        });

        return {getMetadata: function() { return {name: 'Kosatka video embeds'}; }};
    });
})();
