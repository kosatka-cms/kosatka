/*
 * The admin pages' shared form helpers, driven by data attributes
 * (admin/header.html loads this on every admin page).
 *
 * - data-show-when="<field id>=<value>" - the element is shown only while
 *   the field (a select or an input) with that id has that value (one of
 *   several: "<id>=a|b|c").
 * - <select data-filter-by="<select id>"> - its options with
 *   data-filter-value are offered only while that select has that value;
 *   a selected option that goes away resets the select to "".
 * - <tbody data-sortable data-order-input="<input id>"> - its rows with
 *   data-sort-id are dragged to a new order (by the row, or by its
 *   .drag-handle when the row isn't draggable itself); the hidden input
 *   gets the ids, comma-separated, after a drag and when the form is
 *   submitted.
 * - <tbody data-tree-sortable data-order-input="<input id>"> - a tree
 *   (terms, menu links): rows data-sort-id, data-parent-id, data-depth,
 *   rendered depth first; a row is dragged with its subtree, before or
 *   after another row on its level, or (data-tree-nest) into the middle of
 *   a row as its child; the row's <select data-tree-parent> follows, and
 *   picking a parent there moves the row under it. data-root-parent - the
 *   select's value for no parent (default ""), data-indent - the pixels
 *   per level of the row's [data-tree-indent] cell (default 20), set as
 *   its --tree-indent (the cell's padding adds it to its own).
 * - <tbody data-group-sortable data-order-input="<input id>"> - fields
 *   with groups: a group's header row (data-group-header) is dragged with
 *   its members (data-member-of="<header's data-sort-id>"), groups don't
 *   nest; a field dropped on a group's header or member joins it, dropped
 *   elsewhere leaves it; its <select data-group-select> follows (and
 *   moves it when changed). data-member-class - the member rows' class.
 *   All three drag with Kosatka.sortable (app.js); the blocks' regions
 *   have their own script on it (admin_blocks.js).
 * - <form data-autosubmit> - submitted as soon as one of its selects or
 *   checkboxes changes (filters of a list).
 * - [data-focal-picker] - an image's focal point: a click on its
 *   [data-focal-image] puts the point there (its hidden inputs focal_x,
 *   focal_y - percent; [data-focal-dot] marks it), [data-focal-clear]
 *   removes it; each [data-focal-crop] box shows the image cropped as a
 *   style cropping at the point would (centered on it, as far as the
 *   image allows; no point - the center).
 * - <button data-fill-target="<input id>" data-fill-value="…"> - puts the
 *   value into that input (a suggestion, e.g. a link field's sites).
 * - <button data-close-details> - closes the <details> it is in (a form
 *   folded in one: its Cancel), the focus back on its summary.
 * - <button data-insert-token="…"> - puts the text at the cursor of the
 *   [data-token-target] field edited last (the first one before any), the
 *   placeholders of an email.
 * - <input data-date-example="<element id>" data-date-names="{…}"> - a
 *   date format's pattern: the element shows today in it as it is typed
 *   (the pattern letters of pkg/dateformat; the names - F, f, M of the
 *   months, l, D of the weekdays - in the site's language).
 * - .menu-link-form - a menu link's target (menus/link_target_fields.html):
 *   the fields of the type chosen, the content and term searches.
 * - .admin-sidebar-toggle - on a narrow screen opens and closes the menu
 *   (#admin-sidebar) over the page; Escape or a click beside it closes it.
 */
(function () {
    function showWhen() {
        document.querySelectorAll('[data-show-when]').forEach(function (el) {
            var rule = el.getAttribute('data-show-when');
            var eq = rule.indexOf('=');
            var field = document.getElementById(rule.slice(0, eq));
            if (!field) return;
            el.style.display = rule.slice(eq + 1).split('|').indexOf(field.value) !== -1 ? '' : 'none';
        });
    }

    function filterOptions(select) {
        var by = document.getElementById(select.getAttribute('data-filter-by'));
        if (!by) return;
        Array.prototype.forEach.call(select.options, function (o) {
            if (!o.hasAttribute('data-filter-value')) return;
            o.hidden = o.disabled = o.getAttribute('data-filter-value') !== by.value;
        });
        if (select.selectedOptions[0] && select.selectedOptions[0].disabled) {
            select.value = '';
        }
    }

    function filterAll() {
        document.querySelectorAll('select[data-filter-by]').forEach(filterOptions);
    }

    document.addEventListener('change', function (e) {
        if (!e.target.id) return;
        showWhen();
        filterAll();
    });

    function rowsOf(tbody) {
        return Array.prototype.slice.call(tbody.querySelectorAll('tr[data-sort-id]'));
    }

    function idsOf(tbody) {
        return rowsOf(tbody).map(function (row) { return row.getAttribute('data-sort-id'); }).join(',');
    }

    // dragRows - the drag of tbody's rows (Kosatka.sortable): group picks
    // the rows moved with the row dragged, over places them (none - before
    // or after the row hovered); after a drag and on the form's submit the
    // order goes into data-order-input.
    function dragRows(tbody, group, over) {
        var input = document.getElementById(tbody.getAttribute('data-order-input'));
        function done() {
            if (input) input.value = idsOf(tbody);
        }
        Kosatka.sortable(tbody, {items: 'tr[data-sort-id]', group: group, over: over, end: done});
        if (input && input.form) input.form.addEventListener('submit', done);
    }

    function treeSortable(tbody) {
        var nest = tbody.hasAttribute('data-tree-nest');
        var root = tbody.getAttribute('data-root-parent') || '';
        var indent = parseInt(tbody.getAttribute('data-indent') || '20', 10);

        function depthOf(row) { return parseInt(row.getAttribute('data-depth'), 10) || 0; }
        function byID(id) { return tbody.querySelector('tr[data-sort-id="' + id + '"]'); }
        // A row's subtree: the following rows deeper than it (the tree is
        // rendered depth first).
        function blockOf(row) {
            var block = [row];
            for (var next = row.nextElementSibling; next && depthOf(next) > depthOf(row); next = next.nextElementSibling) {
                block.push(next);
            }
            return block;
        }
        function place(block, anchor, parentID, depth) {
            var delta = depth - depthOf(block[0]);
            block.forEach(function (r) {
                tbody.insertBefore(r, anchor);
                if (delta) {
                    var d = depthOf(r) + delta;
                    r.setAttribute('data-depth', d);
                    var cell = r.querySelector('[data-tree-indent]');
                    if (cell) cell.style.setProperty('--tree-indent', (d * indent) + 'px');
                }
            });
            block[0].setAttribute('data-parent-id', parentID);
            var select = block[0].querySelector('select[data-tree-parent]');
            if (select) select.value = parentID;
        }

        dragRows(tbody, blockOf, function (block, row, y) {
            var after = blockOf(row);
            if (nest && y > 0.25 && y < 0.75) {
                place(block, row.nextSibling, row.getAttribute('data-sort-id'), depthOf(row) + 1);
            } else if (y < (nest ? 0.25 : 0.5)) {
                place(block, row, row.getAttribute('data-parent-id') || root, depthOf(row));
            } else {
                place(block, after[after.length - 1].nextSibling, row.getAttribute('data-parent-id') || root, depthOf(row));
            }
        });

        // Picking a parent moves the row with its subtree after the
        // parent's children (no parent - to the end).
        tbody.querySelectorAll('select[data-tree-parent]').forEach(function (select) {
            select.addEventListener('change', function () {
                var row = select.closest('tr');
                var block = blockOf(row);
                if (!select.value || select.value === root) {
                    place(block, null, root, 0);
                    return;
                }
                var parent = byID(select.value);
                if (!parent || block.indexOf(parent) !== -1) return;
                var children = blockOf(parent);
                place(block, children[children.length - 1].nextSibling, select.value, depthOf(parent) + 1);
            });
        });
    }

    function groupSortable(tbody) {
        var memberClass = tbody.getAttribute('data-member-class');
        function byID(id) { return tbody.querySelector('tr[data-sort-id="' + id + '"]'); }
        function blockOf(row) {
            if (!row.hasAttribute('data-group-header')) return [row];
            var id = row.getAttribute('data-sort-id');
            var block = [row];
            for (var next = row.nextElementSibling; next && next.getAttribute('data-member-of') === id; next = next.nextElementSibling) {
                block.push(next);
            }
            return block;
        }
        // setGroup - a row's membership (its attribute, class, handle and
        // select), not its place.
        function setGroup(row, groupID) {
            var handle = row.querySelector('.drag-handle');
            var select = row.querySelector('select[data-group-select]');
            if (groupID) {
                row.setAttribute('data-member-of', groupID);
                if (memberClass) row.classList.add(memberClass);
                if (handle) {
                    handle.innerHTML = '&#8631;';
                    handle.title = Kosatka.t ? Kosatka.t('Drag to reorder or move the field out of the group') : '';
                }
            } else {
                row.removeAttribute('data-member-of');
                if (memberClass) row.classList.remove(memberClass);
                if (handle) {
                    handle.innerHTML = '&#9776;';
                    handle.title = Kosatka.t ? Kosatka.t('Drag to reorder or add the field to a group') : '';
                }
            }
            if (select) select.value = groupID || '0';
        }

        dragRows(tbody, blockOf, function (block, row, y) {
            if (block[0].hasAttribute('data-group-header')) {
                // A group goes before or after another group or field as a
                // whole, never into one: hovering a group snaps to its ends.
                var target = row.hasAttribute('data-member-of') ? byID(row.getAttribute('data-member-of')) : row;
                var targetBlock = blockOf(target);
                var first = targetBlock[0].getBoundingClientRect(), last = targetBlock[targetBlock.length - 1].getBoundingClientRect();
                var rowRect = row.getBoundingClientRect();
                var yAll = (rowRect.top + y * rowRect.height - first.top) / ((last.bottom - first.top) || 1);
                var anchor = yAll > 0.5 ? targetBlock[targetBlock.length - 1].nextSibling : targetBlock[0];
                block.forEach(function (r) { tbody.insertBefore(r, anchor); });
                return;
            }
            // A field: onto a group's header - its first member; onto a
            // member - next to it, in that group; elsewhere - out of groups.
            var field = block[0];
            if (row.hasAttribute('data-group-header')) {
                tbody.insertBefore(field, row.nextSibling);
                setGroup(field, row.getAttribute('data-sort-id'));
                return;
            }
            tbody.insertBefore(field, y > 0.5 ? row.nextSibling : row);
            setGroup(field, row.getAttribute('data-member-of'));
        });

        tbody.querySelectorAll('select[data-group-select]').forEach(function (select) {
            select.addEventListener('change', function () {
                var row = select.closest('tr');
                var groupID = select.value && select.value !== '0' ? select.value : null;
                // Into a group - after its last member; out of one - right
                // after the group it leaves.
                var header = byID(groupID || row.getAttribute('data-member-of'));
                if (groupID && !header) return;
                if (header) {
                    var block = blockOf(header).filter(function (r) { return r !== row; });
                    tbody.insertBefore(row, block[block.length - 1].nextSibling);
                }
                setGroup(row, groupID);
            });
        });
    }

    // menuLinkForm - a menu link's target: the fields of the type chosen;
    // the content and term searches (Kosatka.autocomplete) put the id in
    // their hidden inputs.
    function menuLinkForm(root) {
        var typeSelect = root.querySelector('.menu-link-type-select');
        if (!typeSelect) return;

        function applyType() {
            var type = typeSelect.value;
            root.querySelectorAll('.menu-link-target-fields').forEach(function (div) {
                div.style.display = (div.dataset.for === type) ? '' : 'none';
            });
        }
        applyType();
        typeSelect.addEventListener('change', applyType);

        // setupSearch - the input's suggestions (Kosatka.autocomplete) from
        // url(query) (none - nothing to search yet); a pick puts the id in
        // the hidden input.
        function setupSearch(inputSel, hiddenSel, url, label) {
            var input = root.querySelector(inputSel);
            var hidden = root.querySelector(hiddenSel);
            if (!input || !hidden) return;
            Kosatka.autocomplete(input, {
                source: function (q) {
                    var u = url(q);
                    if (!u) return [];
                    return fetch(u).then(function (resp) { return resp.json(); }).then(function (data) {
                        return data.nodes || data.suggestions || [];
                    });
                },
                label: label,
                empty: Kosatka.t('Nothing found'),
                pick: function (item) {
                    hidden.value = item.id;
                    input.value = label(item);
                }
            });
        }

        setupSearch('.menu-link-node-search', '.menu-link-node-id',
            function (q) { return '/api/nodes/search?q=' + encodeURIComponent(q); },
            function (item) { return item.title + ' (' + item.type + ')'; }
        );

        var vocabSelect = root.querySelector('.menu-link-vocabulary-select');
        setupSearch('.menu-link-term-search', '.menu-link-term-id',
            function (q) {
                if (!vocabSelect || !vocabSelect.value) return null;
                return '/api/taxonomy/autocomplete?vid=' + vocabSelect.value + '&q=' + encodeURIComponent(q);
            },
            function (item) { return item.name; }
        );
    }

    // dateExample - today in the pattern typed into input.
    function dateExample(input) {
        var example = document.getElementById(input.getAttribute('data-date-example'));
        var names = {};
        try {
            names = JSON.parse(input.getAttribute('data-date-names') || '{}');
        } catch (e) {
            return;
        }
        function pad(n) { return n < 10 ? '0' + n : String(n); }
        function format(d, p) {
            var out = '';
            for (var i = 0; i < p.length; i++) {
                var c = p[i];
                if (c === '\\') { if (i + 1 < p.length) { out += p[++i]; } continue; }
                switch (c) {
                    case 'd': out += pad(d.getDate()); break;
                    case 'j': out += d.getDate(); break;
                    case 'm': out += pad(d.getMonth() + 1); break;
                    case 'n': out += d.getMonth() + 1; break;
                    case 'F': case 'f': case 'M': out += (names[c] || [])[d.getMonth()] || ''; break;
                    case 'Y': out += d.getFullYear(); break;
                    case 'y': out += pad(d.getFullYear() % 100); break;
                    case 'H': out += pad(d.getHours()); break;
                    case 'G': out += d.getHours(); break;
                    case 'h': out += pad(d.getHours() % 12 || 12); break;
                    case 'g': out += d.getHours() % 12 || 12; break;
                    case 'A': out += d.getHours() < 12 ? 'AM' : 'PM'; break;
                    case 'a': out += d.getHours() < 12 ? 'am' : 'pm'; break;
                    case 'l': case 'D': out += (names[c] || [])[d.getDay()] || ''; break;
                    case 'N': out += d.getDay() || 7; break;
                    case 'i': out += pad(d.getMinutes()); break;
                    case 's': out += pad(d.getSeconds()); break;
                    default: out += c;
                }
            }
            return out;
        }
        if (!example) return;
        input.addEventListener('input', function () { example.textContent = format(new Date(), input.value); });
    }

    document.addEventListener('DOMContentLoaded', function () {
        showWhen();
        filterAll();
        document.querySelectorAll('input[data-date-example]').forEach(dateExample);
        document.querySelectorAll('.menu-link-form').forEach(menuLinkForm);
        document.querySelectorAll('tbody[data-sortable]').forEach(function (tbody) { dragRows(tbody); });
        document.querySelectorAll('tbody[data-tree-sortable]').forEach(treeSortable);
        document.querySelectorAll('tbody[data-group-sortable]').forEach(groupSortable);
        document.querySelectorAll('[data-focal-picker]').forEach(focalPicker);
    });

    // An image's focal point picked by a click.
    function focalPicker(box) {
        var image = box.querySelector('[data-focal-image]');
        var dot = box.querySelector('[data-focal-dot]');
        var x = box.querySelector('input[name="focal_x"]');
        var y = box.querySelector('input[name="focal_y"]');
        var clear = box.querySelector('[data-focal-clear]');
        if (!image || !x || !y) return;
        function point() {
            return x.value === '' ? null : {x: +x.value / 100, y: +y.value / 100};
        }
        // A crop box: the image covering it, moved so that the point is in
        // its middle without leaving a gap (the server's crop, fillAt).
        function crop(el, p) {
            var img = el.querySelector('img');
            var bw = el.clientWidth, bh = el.clientHeight;
            var iw = img.naturalWidth, ih = img.naturalHeight;
            if (!bw || !bh || !iw || !ih) return;
            var scale = Math.max(bw / iw, bh / ih);
            var w = iw * scale, h = ih * scale;
            var at = p || {x: 0.5, y: 0.5};
            var left = Math.min(0, Math.max(bw - w, bw / 2 - at.x * w));
            var top = Math.min(0, Math.max(bh - h, bh / 2 - at.y * h));
            img.style.objectPosition = left + 'px ' + top + 'px';
        }
        function show() {
            var p = point();
            if (dot) {
                dot.hidden = !p;
                if (p) {
                    dot.style.left = (p.x * 100) + '%';
                    dot.style.top = (p.y * 100) + '%';
                }
            }
            if (clear) clear.hidden = !p;
            box.querySelectorAll('[data-focal-crop]').forEach(function (el) { crop(el, p); });
        }
        image.addEventListener('click', function (e) {
            var r = image.getBoundingClientRect();
            x.value = Math.round(Math.min(1, Math.max(0, (e.clientX - r.left) / r.width)) * 100);
            y.value = Math.round(Math.min(1, Math.max(0, (e.clientY - r.top) / r.height)) * 100);
            show();
        });
        if (clear) {
            clear.addEventListener('click', function () {
                x.value = y.value = '';
                show();
            });
        }
        box.querySelectorAll('[data-focal-crop] img').forEach(function (img) {
            if (!img.complete) img.addEventListener('load', show);
        });
        show();
    }

    // A suggestion put into its input.
    document.addEventListener('click', function (e) {
        var btn = e.target.closest('[data-fill-target]');
        if (!btn) return;
        var input = document.getElementById(btn.getAttribute('data-fill-target'));
        if (!input) return;
        input.value = btn.getAttribute('data-fill-value') || '';
        input.focus();
    });

    // A folded form's Cancel.
    document.addEventListener('click', function (e) {
        var btn = e.target.closest('[data-close-details]');
        if (!btn) return;
        var details = btn.closest('details');
        if (!details) return;
        details.open = false;
        var summary = details.querySelector('summary');
        if (summary) summary.focus();
    });

    // A placeholder put at the cursor of the field edited last.
    var tokenTarget = null;
    document.addEventListener('focusin', function (e) {
        if (e.target.matches && e.target.matches('[data-token-target]')) tokenTarget = e.target;
    });
    document.addEventListener('click', function (e) {
        var btn = e.target.closest('[data-insert-token]');
        if (!btn) return;
        var field = tokenTarget || document.querySelector('[data-token-target]');
        if (!field) return;
        var token = btn.getAttribute('data-insert-token');
        var start = field.selectionStart == null ? field.value.length : field.selectionStart;
        var end = field.selectionEnd == null ? start : field.selectionEnd;
        field.value = field.value.slice(0, start) + token + field.value.slice(end);
        field.focus();
        field.setSelectionRange(start + token.length, start + token.length);
    });

    // Filters applied when a select changes.
    document.addEventListener('change', function (e) {
        var form = e.target.closest('form[data-autosubmit]');
        if (form && (e.target.tagName === 'SELECT' || e.target.type === 'checkbox')) form.submit();
    });

    // The admin menu on a narrow screen.
    (function () {
        var toggle = document.querySelector('.admin-sidebar-toggle');
        var sidebar = document.getElementById('admin-sidebar');
        if (!toggle || !sidebar) return;
        function setOpen(open) {
            document.body.classList.toggle('admin-sidebar-open', open);
            toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
        }
        toggle.addEventListener('click', function (e) {
            e.stopPropagation();
            setOpen(!document.body.classList.contains('admin-sidebar-open'));
        });
        document.addEventListener('click', function (e) {
            if (document.body.classList.contains('admin-sidebar-open') && !sidebar.contains(e.target)) setOpen(false);
        });
        document.addEventListener('keydown', function (e) {
            if (e.key === 'Escape' && document.body.classList.contains('admin-sidebar-open')) {
                setOpen(false);
                toggle.focus();
            }
        });
    })();
})();
