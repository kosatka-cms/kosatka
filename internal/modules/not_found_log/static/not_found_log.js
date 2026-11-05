// 404 log module, loaded after app.js (attached by its template,
// {{attach}}): a row's redirect form opens under it, offering the content
// whose title matches the path's last word.
(function initNotFoundLog() {
    // The path's last word: its last segment, without an extension and
    // with dashes and underscores as spaces.
    function lastWord(path) {
        const parts = path.split('/').filter(Boolean);
        let word = parts.length ? parts[parts.length - 1] : '';
        try {
            word = decodeURIComponent(word);
        } catch (e) {
            // The raw segment then.
        }
        return word.replace(/\.[a-z0-9]+$/i, '').replace(/[-_]+/g, ' ').trim();
    }

    function suggest(box) {
        if (box.dataset.loaded) {
            return;
        }
        box.dataset.loaded = '1';
        const q = lastWord(box.dataset.notFoundSuggest || '');
        if (q.length < 3) {
            return;
        }
        const input = box.closest('form').querySelector('input[name="to"]');
        fetch('/api/nodes/search?q=' + encodeURIComponent(q), {credentials: 'same-origin'})
            .then(function(res) { return res.ok ? res.json() : {nodes: []}; })
            .then(function(data) {
                const nodes = (data.nodes || []).slice(0, 5);
                if (!nodes.length) {
                    return;
                }
                const label = document.createElement('span');
                label.textContent = box.dataset.label;
                box.appendChild(label);
                nodes.forEach(function(n) {
                    const chip = document.createElement('button');
                    chip.type = 'button';
                    chip.className = 'not-found-chip';
                    chip.textContent = n.title + ' · ' + n.url;
                    chip.addEventListener('click', function() {
                        input.value = n.url;
                        input.focus();
                    });
                    box.appendChild(chip);
                });
                box.hidden = false;
            })
            .catch(function() {});
    }

    document.querySelectorAll('[data-not-found-toggle]').forEach(function(btn) {
        btn.addEventListener('click', function() {
            const row = document.getElementById(btn.dataset.notFoundToggle);
            if (!row) {
                return;
            }
            row.hidden = !row.hidden;
            btn.setAttribute('aria-expanded', String(!row.hidden));
            if (!row.hidden) {
                const box = row.querySelector('[data-not-found-suggest]');
                if (box) {
                    suggest(box);
                }
                row.querySelector('input[name="to"]').focus();
            }
        });
    });
})();
