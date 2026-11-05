// Video embed module: the content form's link fields the module shows
// (modulekit.FieldFormatter.EditorScript; a field's wrapper
// [data-field-links] with data-field-formatter "video_embed_…"): under a
// pasted or typed address ([data-field-link-preview], the core's box),
// its video's preview, title and service - or a note that it isn't a
// video link the site shows.
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

    // show - the row's preview for its address.
    function show(row) {
        const input = row.querySelector('[data-field-link-url]');
        const box = row.querySelector('[data-field-link-preview]');
        if (!input || !box) {
            return;
        }
        let url = input.value.trim();
        if (url && url.indexOf('://') < 0 && url.indexOf('.') > 0) {
            url = 'https://' + url;
        }
        if (!/^https?:\/\/\S+$/.test(url)) {
            box.hidden = true;
            box.textContent = '';
            return;
        }
        box.dataset.url = url;
        resolve(url).then(function(info) {
            if (box.dataset.url !== url) {
                return;
            }
            box.textContent = '';
            box.hidden = false;
            box.classList.toggle('field-link-preview--warn', !info.ok);
            if (!info.ok) {
                box.textContent = Kosatka.t('Not a video link the site shows: it will be shown as a link.');
                return;
            }
            const thumb = document.createElement(info.preview ? 'img' : 'span');
            thumb.className = 'field-link-preview-thumb';
            if (info.preview) {
                thumb.src = info.preview;
                thumb.alt = '';
            }
            const text = document.createElement('span');
            text.className = 'field-link-preview-text';
            const title = document.createElement('strong');
            title.textContent = info.title || info.provider;
            const meta = document.createElement('span');
            meta.textContent = info.provider;
            text.append(title, meta);
            box.append(thumb, text);
        });
    }

    function attach(field) {
        if (field.dataset.videoEmbedField) {
            return;
        }
        field.dataset.videoEmbedField = '1';
        field.querySelectorAll('[data-field-link]').forEach(show);
        let timer;
        field.addEventListener('input', function(e) {
            const row = e.target.closest('[data-field-link]');
            if (!row || !e.target.matches('[data-field-link-url]')) {
                return;
            }
            clearTimeout(timer);
            timer = setTimeout(function() { show(row); }, 400);
        });
        field.addEventListener('field-row-added', function(e) {
            show(e.target);
        });
    }

    function init() {
        document.querySelectorAll('[data-field-links][data-field-formatter^="video_embed"]').forEach(attach);
    }
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
