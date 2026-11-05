// fieldImages - an image field with several images in the content form
// ([data-field-images]): ↑/↓ ([data-field-image-move] = -1 / 1) move an
// image's row ([data-field-image]); the rows' hidden inputs, in their new
// order, are the order saved.
Kosatka.behaviors.fieldImages = {
    attach: function (context) {
        Kosatka.once('field-images', '[data-field-images]', context).forEach(function (list) {
            list.addEventListener('click', function (e) {
                var button = e.target.closest('[data-field-image-move]');
                if (!button) return;
                var row = button.closest('[data-field-image]');
                if (button.getAttribute('data-field-image-move') === '-1') {
                    var prev = row.previousElementSibling;
                    if (prev) list.insertBefore(row, prev);
                } else {
                    var next = row.nextElementSibling;
                    if (next) list.insertBefore(next, row);
                }
                button.focus();
            });
        });
    }
};

// fieldEmails - an e-mail field's address hidden from harvesters
// (fields/email.html: data-email, the address backwards; the text "info
// [at] example.com"): the link and its text made in the browser.
Kosatka.behaviors.fieldEmails = {
    attach: function (context) {
        Kosatka.once('field-email', 'a[data-email]', context).forEach(function (link) {
            var address = link.getAttribute('data-email').split('').reverse().join('');
            link.href = 'mailto:' + address;
            link.textContent = address;
        });
    }
};

// fieldRows - a field with several values in rows in the content form
// (links, dates; [data-field-rows]): ↑/↓ ([data-field-row-move]) move a
// row ([data-field-row]), ✕ ([data-field-row-remove]) removes it (the
// last one is emptied instead), "Add" ([data-field-row-add]) adds a row
// from the <template data-field-row-template> up to data-max (0 - no
// limit). The rows' inputs, in their order, are the order saved. A new
// row fires field-row-added on it (a module's script hooks there).
Kosatka.behaviors.fieldRows = {
    attach: function (context) {
        Kosatka.once('field-rows', '[data-field-rows]', context).forEach(function (field) {
            var rows = field.querySelector('[data-field-row-list]');
            var template = field.querySelector('template[data-field-row-template]');
            var add = field.querySelector('[data-field-row-add]');
            var max = parseInt(field.getAttribute('data-max'), 10) || 0;
            function count() {
                return rows.querySelectorAll('[data-field-row]').length;
            }
            function update() {
                if (add) add.hidden = max > 0 && count() >= max;
            }
            field.addEventListener('click', function (e) {
                var move = e.target.closest('[data-field-row-move]');
                var remove = e.target.closest('[data-field-row-remove]');
                if (move) {
                    var row = move.closest('[data-field-row]');
                    if (move.getAttribute('data-field-row-move') === '-1') {
                        if (row.previousElementSibling) rows.insertBefore(row, row.previousElementSibling);
                    } else if (row.nextElementSibling) {
                        rows.insertBefore(row.nextElementSibling, row);
                    }
                    move.focus();
                } else if (remove) {
                    var gone = remove.closest('[data-field-row]');
                    if (count() > 1) {
                        gone.remove();
                    } else {
                        gone.querySelectorAll('input').forEach(function (input) { input.value = ''; });
                        gone.querySelectorAll('[data-field-link-preview]').forEach(function (preview) { preview.hidden = true; preview.textContent = ''; });
                    }
                    update();
                } else if (add && e.target.closest('[data-field-row-add]') && template) {
                    var added = template.content.firstElementChild.cloneNode(true);
                    rows.appendChild(added);
                    update();
                    var input = added.querySelector('input');
                    if (input) input.focus();
                    added.dispatchEvent(new CustomEvent('field-row-added', {bubbles: true}));
                }
            });
            update();
        });
    }
};

// fieldBlocks - a blocks field in the content form ([data-field-blocks]):
// its blocks ([data-field-block], cards in [data-field-block-list]) - ↑/↓
// ([data-field-block-move]) move one, ✕ ([data-field-block-remove])
// removes it; "Add: <set>" ([data-field-block-add]) adds a block of that
// set from its <template data-field-block-template>, its key (the
// template's __KEY__, in its inputs' names) made here, up to data-max (0 -
// no limit). The blocks' order is the order saved. A block's text
// editors are taken off before it moves and put back after (an editor's
// frame doesn't survive the move); its heading's summary follows its
// first texts.
Kosatka.behaviors.fieldBlocks = {
    attach: function (context) {
        Kosatka.once('field-blocks', '[data-field-blocks]', context).forEach(function (field) {
            var list = field.querySelector('[data-field-block-list]');
            var max = parseInt(field.getAttribute('data-max'), 10) || 0;
            var made = 0;
            function blocks() {
                return Array.prototype.filter.call(list.children, function (el) { return el.matches('[data-field-block]'); });
            }
            function update() {
                var full = max > 0 && blocks().length >= max;
                var adds = field.querySelector('.field-blocks-add');
                if (adds) adds.hidden = full;
            }
            function editors(block, on) {
                block.querySelectorAll('.text-format-picker').forEach(function (picker) {
                    picker.dispatchEvent(new Event(on ? 'change' : 'text-format-detach'));
                });
            }
            function summary(block) {
                var out = block.querySelector('[data-field-block-summary]');
                if (!out) return;
                var parts = [];
                block.querySelectorAll('.field-block-body input[type="text"], .field-block-body textarea').forEach(function (input) {
                    // An inert document: the text of HTML typed, nothing loaded.
                    var doc = new DOMParser().parseFromString(input.value, 'text/html');
                    var text = (doc.body.textContent || '').replace(/\s+/g, ' ').trim();
                    if (text && parts.length < 2) parts.push(text);
                });
                var s = parts.join(' - ');
                out.textContent = s.length > 120 ? s.slice(0, 119) + '…' : s;
            }
            field.addEventListener('click', function (e) {
                var move = e.target.closest('[data-field-block-move]');
                var remove = e.target.closest('[data-field-block-remove]');
                var add = e.target.closest('[data-field-block-add]');
                if (move) {
                    e.preventDefault();
                    var block = move.closest('[data-field-block]');
                    var other = move.getAttribute('data-field-block-move') === '-1' ? block.previousElementSibling : block.nextElementSibling;
                    if (!other) return;
                    editors(block, false);
                    editors(other, false);
                    if (other === block.previousElementSibling) {
                        list.insertBefore(block, other);
                    } else {
                        list.insertBefore(other, block);
                    }
                    editors(block, true);
                    editors(other, true);
                    move.focus();
                } else if (remove) {
                    e.preventDefault();
                    var gone = remove.closest('[data-field-block]');
                    editors(gone, false);
                    gone.remove();
                    update();
                } else if (add) {
                    var template = field.querySelector('template[data-field-block-template="' + add.getAttribute('data-field-block-add') + '"]');
                    if (!template) return;
                    var key = 'n' + Date.now().toString(36) + (++made);
                    var holder = document.createElement('div');
                    holder.innerHTML = template.innerHTML.split('__KEY__').join(key);
                    var added = holder.firstElementChild;
                    list.appendChild(added);
                    Kosatka.attachBehaviors(added);
                    update();
                    var input = added.querySelector('.field-block-body input:not([type="hidden"]), .field-block-body textarea, .field-block-body select');
                    if (input) input.focus();
                }
            });
            field.addEventListener('input', function (e) {
                var block = e.target.closest('[data-field-block]');
                if (block) summary(block);
            });
            update();
        });
    }
};

// A field the browser finds invalid on submitting (required, too long…)
// inside a folded <details> - a form group of the content form, a setting
// of its side column - opens it, so the browser can show the field.
document.addEventListener('invalid', function (e) {
    var details = e.target.closest && e.target.closest('details');
    while (details) {
        details.open = true;
        details = details.parentElement && details.parentElement.closest('details');
    }
}, true);

// settingsFolds - settings in groups that fold ([data-settings-fold], a
// <details> of .admin-setting rows): a group's line shows its values
// ([data-settings-summary]), kept as they change - a switch on (its
// label), a select's option (its data-summary, else its text; "" - none),
// a radio's label, a text's value, a number with its label; a row with
// data-summary="off" is left out. [data-settings-fold-all] opens or folds
// them all (its data-expand / data-collapse - its words). A group the
// address's #fragment names opens (a link to it on the page too).
Kosatka.behaviors.settingsFolds = {
    attach: function (context) {
        Kosatka.once('settings-fold-hash', 'body', context).forEach(function () {
            var openNamed = function () {
                var id = window.location.hash.slice(1);
                var group = id && document.getElementById(id);
                if (group && group.matches('[data-settings-fold]')) {
                    group.open = true;
                }
            };
            openNamed();
            window.addEventListener('hashchange', openNamed);
        });
        var cut = function (v) { return v.length > 48 ? v.slice(0, 47) + '…' : v; };
        var summary = function (group) {
            var parts = [];
            group.querySelectorAll('.admin-setting').forEach(function (row) {
                if (row.getAttribute('data-summary') === 'off') return;
                var labelEl = row.querySelector('.admin-setting-label');
                var label = labelEl ? labelEl.textContent.trim() : '';
                var control = row.querySelector('.admin-setting-control') || row;
                var select = control.querySelector('select');
                var radios = control.querySelectorAll('input[type="radio"]');
                var check = control.querySelector('input[type="checkbox"]');
                var text = control.querySelector('input[type="text"], input[type="number"], input:not([type]), textarea');
                var v = '';
                if (select) {
                    var o = select.selectedOptions[0];
                    if (o && o.value !== '') v = o.getAttribute('data-summary') || o.textContent.trim();
                } else if (radios.length) {
                    var r = Array.prototype.find.call(radios, function (x) { return x.checked; });
                    var rl = r && control.querySelector('label[for="' + r.id + '"]');
                    if (r) v = r.getAttribute('data-summary') || (rl ? rl.textContent.trim() : r.value);
                } else if (check) {
                    if (check.checked) v = label;
                } else if (text && text.value.trim() !== '') {
                    v = text.type === 'number' ? label + ': ' + text.value.trim() : text.value.trim();
                }
                if (v) parts.push(cut(v.replace(/\s+/g, ' ')));
            });
            return parts.join(' · ');
        };
        Kosatka.once('settings-fold', '[data-settings-fold]', context).forEach(function (group) {
            var line = group.querySelector('[data-settings-summary]');
            if (!line) return;
            var update = function () { line.textContent = summary(group); };
            update();
            group.addEventListener('input', update);
            group.addEventListener('change', update);
        });
        Kosatka.once('settings-fold-all', '[data-settings-fold-all]', context).forEach(function (button) {
            var groups = function () { return document.querySelectorAll('[data-settings-fold]'); };
            var sync = function () {
                var allOpen = Array.prototype.every.call(groups(), function (g) { return g.open; });
                button.textContent = button.getAttribute(allOpen ? 'data-collapse' : 'data-expand');
            };
            button.addEventListener('click', function () {
                var open = !Array.prototype.every.call(groups(), function (g) { return g.open; });
                groups().forEach(function (g) { g.open = open; });
                sync();
            });
            groups().forEach(function (g) { g.addEventListener('toggle', sync); });
            sync();
        });
    }
};

// fieldImageSingle - an image field with one image in the content form
// ([data-field-image-single]): a file picked (a click or a drop on the
// empty row) shows in the preview, the alt text input appears and the row
// stops being a drop area.
Kosatka.behaviors.fieldImageSingle = {
    attach: function (context) {
        Kosatka.once('field-image-single', '[data-field-image-single]', context).forEach(function (field) {
            var input = field.querySelector('input[type="file"]');
            if (!input) return;
            input.addEventListener('change', function () {
                var file = input.files && input.files[0];
                if (!file) return;
                var preview = field.querySelector('[data-field-image-preview]');
                if (preview && preview.tagName !== 'IMG') {
                    var img = document.createElement('img');
                    img.className = 'field-image-thumb';
                    img.alt = '';
                    img.setAttribute('data-field-image-preview', '');
                    preview.replaceWith(img);
                    preview = img;
                }
                if (preview) preview.src = URL.createObjectURL(file);
                var dropText = field.querySelector('[data-field-image-drop-text]');
                if (dropText) dropText.remove();
                var alt = field.querySelector('[data-field-image-alt]');
                if (alt) alt.hidden = false;
                field.classList.remove('is-empty');
            });
        });
    }
};

