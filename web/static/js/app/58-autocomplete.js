// ===== Autocomplete =====
// Kosatka.autocomplete(input, opts) - suggestions under a text input,
// picked with the mouse or the keyboard (the arrows and Enter; Escape
// closes the list, Tab leaves it). opts.source(query) - the items for
// what is typed (an array or a Promise of one), asked once opts.minChars
// are typed (1 by default) and the typing pauses for opts.delay ms (250);
// opts.onFocus - the list also opens when the input gets the focus.
// opts.label(item) - its text; opts.group(item) - the heading it goes
// under; opts.empty - the text when nothing matches (no list without
// it); opts.pick(item) - chosen (it sets the input's value as it needs).
// Enter with the list open picks the item marked, or the first one, and
// doesn't submit the form. The list is ul.kosatka-autocomplete under the
// input (its parent gets .kosatka-autocomplete-wrap), styled by
// overlays.css; the input gets the combobox's ARIA attributes.
var autocompleteCount = 0;

Kosatka.autocomplete = function (input, opts) {
    var minChars = opts.minChars === undefined ? 1 : opts.minChars;
    var delay = opts.delay === undefined ? 250 : opts.delay;
    var id = 'kosatka-autocomplete-' + (++autocompleteCount);
    var list = document.createElement('ul');
    list.className = 'kosatka-autocomplete';
    list.id = id;
    list.setAttribute('role', 'listbox');
    list.hidden = true;
    input.parentNode.classList.add('kosatka-autocomplete-wrap');
    input.insertAdjacentElement('afterend', list);
    input.setAttribute('role', 'combobox');
    input.setAttribute('aria-autocomplete', 'list');
    input.setAttribute('aria-expanded', 'false');
    input.setAttribute('aria-controls', id);
    input.setAttribute('autocomplete', 'off');

    var items = [];
    var active = -1;
    var timer = 0;
    var asked = 0;

    function options() {
        return list.querySelectorAll('[role="option"]');
    }
    function close() {
        list.hidden = true;
        list.textContent = '';
        items = [];
        active = -1;
        input.setAttribute('aria-expanded', 'false');
        input.removeAttribute('aria-activedescendant');
    }
    function mark(i) {
        var all = options();
        if (all[active]) {
            all[active].setAttribute('aria-selected', 'false');
        }
        active = i;
        if (all[i]) {
            all[i].setAttribute('aria-selected', 'true');
            input.setAttribute('aria-activedescendant', all[i].id);
            all[i].scrollIntoView({ block: 'nearest' });
        }
    }
    function choose(i) {
        var item = items[i];
        close();
        if (item !== undefined) {
            opts.pick(item);
        }
    }
    function line(cls, text, role) {
        var li = document.createElement('li');
        li.className = cls;
        li.setAttribute('role', role || 'presentation');
        li.textContent = text;
        list.appendChild(li);
        return li;
    }
    function show(found) {
        close();
        items = found || [];
        if (!items.length && !opts.empty) {
            return;
        }
        if (!items.length) {
            line('kosatka-autocomplete-empty', opts.empty);
        }
        var lastGroup;
        items.forEach(function (item, i) {
            if (opts.group) {
                var group = opts.group(item) || '';
                if (group !== lastGroup) {
                    if (group) {
                        line('kosatka-autocomplete-group', group);
                    }
                    lastGroup = group;
                }
            }
            var li = line('kosatka-autocomplete-item', opts.label(item), 'option');
            li.id = id + '-' + i;
            li.setAttribute('aria-selected', 'false');
            li.dataset.index = i;
        });
        // Right under the input, as wide, wherever it is in its parent.
        list.style.top = (input.offsetTop + input.offsetHeight) + 'px';
        list.style.left = input.offsetLeft + 'px';
        list.style.width = input.offsetWidth + 'px';
        list.hidden = false;
        input.setAttribute('aria-expanded', 'true');
    }
    function ask() {
        clearTimeout(timer);
        var query = input.value.trim();
        var ticket = ++asked;
        if (query.length < minChars) {
            close();
            return;
        }
        Promise.resolve(opts.source(query)).then(function (found) {
            if (ticket === asked) {
                show(found);
            }
        }).catch(function (err) {
            console.error('Autocomplete:', err);
        });
    }

    input.addEventListener('input', function () {
        clearTimeout(timer);
        timer = setTimeout(ask, delay);
    });
    if (opts.onFocus) {
        input.addEventListener('focus', ask);
        input.addEventListener('click', function () {
            if (list.hidden) {
                ask();
            }
        });
    }
    input.addEventListener('blur', function () {
        clearTimeout(timer);
        asked++;
        close();
    });
    input.addEventListener('keydown', function (e) {
        var open = !list.hidden;
        if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
            e.preventDefault();
            if (!open) {
                ask();
                return;
            }
            var n = options().length;
            if (n) {
                mark(e.key === 'ArrowDown' ? Math.min(active + 1, n - 1) : Math.max(active - 1, 0));
            }
        } else if (e.key === 'Enter' && open) {
            e.preventDefault();
            choose(active >= 0 ? active : (items.length ? 0 : -1));
        } else if (e.key === 'Escape' && open) {
            // Only the list: a dialog around the input stays.
            e.preventDefault();
            e.stopPropagation();
            close();
        }
    });
    // mousedown, not click: the input keeps the focus (no blur closing the
    // list first).
    list.addEventListener('mousedown', function (e) {
        e.preventDefault();
        var li = e.target.closest('[role="option"]');
        if (li) {
            choose(parseInt(li.dataset.index, 10));
        }
    });

    return { close: close };
};
