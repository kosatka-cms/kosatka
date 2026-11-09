// ===== The media library picker =====
// An image or file field's "From the media library" ([data-media-picker] -
// the field's ID; data-media-name - its input name; data-media-kind -
// "image" or "file"; data-media-multiple - several images or files): a window of
// the files the field may take (/api/media/pick, found by name or alt
// text), one chosen with its preview and alt text (the file's: the same
// wherever it is used). One - into the field's row and its hidden
// <name>_library (a file uploaded after clears it); several - a new row
// with its <name>_keep, as the field's own images (or files) have.
Kosatka.behaviors.mediaPicker = {
    attach: function (context) {
        Kosatka.once('media-picker', '[data-media-picker]', context).forEach(function (button) {
            var field = button.getAttribute('data-media-picker');
            var name = button.getAttribute('data-media-name');
            var isImage = button.getAttribute('data-media-kind') === 'image';
            var multiple = button.hasAttribute('data-media-multiple');
            var holder = button.closest('.mb-3, .dynamic-field-input, form') || document;
            var picked = holder.querySelector('input[name="' + name + '_library"]');
            var upload = holder.querySelector('input[type="file"][name="' + name + '"]');
            if (upload && picked) {
                upload.addEventListener('change', function () { picked.value = ''; });
            }
            button.addEventListener('click', function () {
                Kosatka.mediaPickerWindow(field, isImage, function (item, alt) {
                    if (multiple) {
                        addRow(holder, name, item, alt, isImage);
                    } else if (isImage) {
                        putImage(holder, name, item, alt, picked, upload);
                    } else {
                        putFile(button, item, picked, upload);
                    }
                }, button);
            });
        });

        function el(tag, cls, text) {
            var e = document.createElement(tag);
            if (cls) e.className = cls;
            if (text) e.textContent = text;
            return e;
        }
        function putImage(holder, name, item, alt, picked, upload) {
            var single = holder.querySelector('[data-field-image-single]');
            if (!single) return;
            picked.value = item.id;
            if (upload) upload.value = '';
            var preview = single.querySelector('[data-field-image-preview]');
            if (preview && preview.tagName !== 'IMG') {
                var img = el('img', 'field-image-thumb');
                img.setAttribute('data-field-image-preview', '');
                preview.replaceWith(img);
                preview = img;
            }
            if (preview) {
                preview.src = item.thumb;
                preview.alt = alt;
            }
            var dropText = single.querySelector('[data-field-image-drop-text]');
            if (dropText) dropText.remove();
            var altBox = single.querySelector('[data-field-image-alt]');
            if (altBox) {
                altBox.hidden = false;
                var altInput = altBox.querySelector('input');
                if (altInput) altInput.value = alt;
            }
            var del = single.querySelector('.field-image-delete-input');
            if (del) del.checked = false;
            single.classList.remove('is-empty');
        }
        function putFile(button, item, picked, upload) {
            picked.value = item.id;
            if (upload) upload.value = '';
            var shown = button.parentNode.querySelector('[data-media-chosen]');
            if (!shown) {
                shown = el('span', 'form-text ms-2');
                shown.setAttribute('data-media-chosen', '');
                button.after(shown);
            }
            shown.textContent = item.name;
        }
        function addRow(holder, name, item, alt, isImage) {
            var list = holder.querySelector('[data-field-images]');
            if (!list || list.querySelector('input[name="' + name + '_keep"][value="' + item.id + '"]')) return;
            var row = el('div', 'field-image-row');
            row.setAttribute('data-field-image', '');
            var keep = el('input');
            keep.type = 'hidden';
            keep.name = name + '_keep';
            keep.value = item.id;
            var shown = [];
            if (isImage) {
                var img = el('img', 'field-image-thumb');
                img.src = item.thumb;
                img.alt = alt;
                var altBox = el('div', 'field-image-alt');
                var altInput = el('input', 'form-control form-control-sm');
                altInput.type = 'text';
                altInput.name = name + '_alt_' + item.id;
                altInput.maxLength = 255;
                altInput.value = alt;
                altInput.setAttribute('aria-label', Kosatka.t('Alt text'));
                altBox.appendChild(altInput);
                shown = [img, altBox];
            } else {
                shown = [el('span', 'field-file-name', item.name)];
            }
            var actions = el('div', 'field-image-actions');
            [['-1', '↑', Kosatka.t('Move up')], ['1', '↓', Kosatka.t('Move down')]].forEach(function (m) {
                var b = el('button', 'field-image-btn', m[1]);
                b.type = 'button';
                b.setAttribute('data-field-image-move', m[0]);
                b.setAttribute('aria-label', m[2]);
                b.title = m[2];
                actions.appendChild(b);
            });
            var remove = el('button', 'field-image-btn field-image-delete', '✕');
            remove.type = 'button';
            remove.title = Kosatka.t('Delete');
            remove.setAttribute('aria-label', Kosatka.t('Delete'));
            remove.addEventListener('click', function () { row.remove(); });
            actions.appendChild(remove);
            row.append.apply(row, [keep].concat(shown, [actions]));
            list.appendChild(row);
        }
    }
};

// Kosatka.mediaPickerWindow(field, isImage, done, opener) - the window of
// the media library's files field may take (Kosatka.window,
// .kosatka-media); done(item, alt) gets the chosen one (its alt text as
// edited there).
Kosatka.mediaPickerWindow = (function () {
    function el(tag, cls, text) {
        var e = document.createElement(tag);
        if (cls) e.className = cls;
        if (text) e.textContent = text;
        return e;
    }
    function size(bytes) {
        if (bytes >= 1048576) return (bytes / 1048576).toFixed(1) + ' MB';
        return Math.max(1, Math.round(bytes / 1024)) + ' KB';
    }

    return function (field, isImage, done, opener) {
        var win = Kosatka.window({
            title: isImage ? Kosatka.t('Choose an image') : Kosatka.t('Choose a file'),
            className: 'kosatka-media',
            opener: opener
        });
        var listCol = el('div', 'kosatka-media-list');
        var search = el('input');
        search.type = 'search';
        search.placeholder = Kosatka.t('Name or alt text');
        search.setAttribute('aria-label', Kosatka.t('Name or alt text'));
        var grid = el('ul', 'kosatka-media-grid');
        grid.setAttribute('role', 'listbox');
        grid.setAttribute('aria-label', Kosatka.t('Media library'));
        var more = el('button', 'kosatka-window-btn kosatka-media-more', Kosatka.t('Show more'));
        more.type = 'button';
        more.hidden = true;
        listCol.append(search, grid, more);
        var side = el('div', 'kosatka-media-side');
        side.setAttribute('aria-live', 'polite');
        win.body.append(listCol, side);
        var cancel = win.button(Kosatka.t('Cancel'));
        var choose = win.button(Kosatka.t('Choose'), true);
        choose.disabled = true;

        var items = {};
        var chosen = null;
        var altInput = null;
        var page = 1;
        var asked = 0;
        var timer = 0;

        function showSide(item) {
            side.textContent = '';
            altInput = null;
            if (!item) {
                side.appendChild(el('p', '', Kosatka.t('Choose a file: its preview shows here.')));
                return;
            }
            if (item.thumb) {
                var img = el('img');
                img.src = item.thumb;
                img.alt = '';
                side.appendChild(img);
            }
            side.appendChild(el('strong', '', item.name));
            var meta = (item.width && item.height ? item.width + '×' + item.height + ' · ' : '') + item.ext + ' · ' + size(item.size);
            side.appendChild(el('p', '', meta));
            side.appendChild(el('p', '', item.uses ? Kosatka.formatPlural(item.uses, 'Used in @count place', 'Used in @count places') : Kosatka.t('Used nowhere')));
            if (isImage) {
                var label = el('label', '', Kosatka.t('Alternative text'));
                label.htmlFor = 'kosatka-media-alt';
                altInput = el('input');
                altInput.id = 'kosatka-media-alt';
                altInput.type = 'text';
                altInput.maxLength = 255;
                altInput.value = item.alt || '';
                side.append(label, altInput, el('p', '', Kosatka.t('The same wherever the image is used.')));
            }
        }
        function select(id) {
            chosen = items[id] || null;
            choose.disabled = !chosen;
            grid.querySelectorAll('li').forEach(function (li) {
                li.setAttribute('aria-selected', String(li.dataset.id) === String(id) ? 'true' : 'false');
            });
            showSide(chosen);
        }
        function load(reset) {
            if (reset) {
                page = 1;
                grid.textContent = '';
                items = {};
                select(null);
            }
            var ticket = ++asked;
            var url = '/api/media/pick?field=' + encodeURIComponent(field) + '&page=' + page + '&q=' + encodeURIComponent(search.value.trim());
            fetch(url, {credentials: 'same-origin', headers: {'Accept': 'application/json'}})
                .then(function (r) { return r.ok ? r.json() : Promise.reject(r.status); })
                .then(function (data) {
                    if (ticket !== asked) return;
                    (data.items || []).forEach(function (item) {
                        items[item.id] = item;
                        var li = el('li');
                        li.setAttribute('role', 'option');
                        li.setAttribute('aria-selected', 'false');
                        li.dataset.id = item.id;
                        if (item.thumb) {
                            var img = el('img');
                            img.src = item.thumb;
                            img.alt = '';
                            img.loading = 'lazy';
                            li.appendChild(img);
                        } else {
                            li.appendChild(el('span', 'kosatka-media-ext', item.ext || '?'));
                        }
                        li.appendChild(el('span', 'kosatka-media-name', item.name));
                        grid.appendChild(li);
                    });
                    if (!grid.children.length) {
                        grid.appendChild(el('li', 'kosatka-media-empty', Kosatka.t('Nothing found.')));
                    }
                    more.hidden = !data.more;
                })
                .catch(function () {
                    if (ticket === asked) {
                        grid.textContent = '';
                        grid.appendChild(el('li', 'kosatka-media-empty', Kosatka.t('Error')));
                    }
                });
        }

        grid.addEventListener('click', function (e) {
            var li = e.target.closest('li[data-id]');
            if (li) select(li.dataset.id);
        });
        grid.addEventListener('dblclick', function (e) {
            var li = e.target.closest('li[data-id]');
            if (li) {
                select(li.dataset.id);
                choose.click();
            }
        });
        search.addEventListener('input', function () {
            clearTimeout(timer);
            timer = setTimeout(function () { load(true); }, 250);
        });
        more.addEventListener('click', function () {
            page++;
            load(false);
        });
        cancel.addEventListener('click', win.close);
        choose.addEventListener('click', function () {
            if (!chosen) return;
            var alt = altInput ? altInput.value.trim() : (chosen.alt || '');
            win.close();
            done(chosen, alt);
        });
        showSide(null);
        load(true);
        search.focus();
    };
})();
