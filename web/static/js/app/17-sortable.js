// ===== Sortable =====
// Kosatka.sortable(container, opts) - a new order by dragging (the
// browser's drag and drop): opts.items - the selector of what moves
// inside container (it, or an element in it, is draggable="true";
// opts.handle - only a drag started there counts). The dragged item goes
// before or after the one under the pointer: by its top or bottom half,
// or (opts.axis "x", a grid) its left or right half. opts.group(item) -
// the items that move with it (a tree's subtree, a group's members; the
// item alone by default); opts.over(moved, item, ratio) places them
// itself (ratio - 0…1 along the axis); opts.into(moved) - over the
// container but not over an item (an empty region); opts.end(moved) once
// dropped. Containers with the same opts.shared exchange items. Listens
// on the container, so items put in later move too; a drag started in a
// form control is refused. The moved items have the class "dragging",
// their table "drag-in-progress" meanwhile.
var sortDrag = null;

Kosatka.sortable = function (container, opts) {
    var axis = opts.axis || 'y';

    function itemOf(el) {
        var item = el.closest && el.closest(opts.items);
        return item && container.contains(item) ? item : null;
    }
    function accepts() {
        return sortDrag && (sortDrag.opts === opts || (opts.shared && sortDrag.opts.shared === opts.shared));
    }
    function marked(on) {
        sortDrag.moved.forEach(function (item) { item.classList.toggle('dragging', on); });
        var table = sortDrag.moved[0].closest('table');
        if (table) table.classList.toggle('drag-in-progress', on);
    }

    container.addEventListener('dragstart', function (e) {
        var item = itemOf(e.target);
        if (!item) {
            return;
        }
        if (e.target.closest('select, input, textarea') || (opts.handle && !e.target.closest(opts.handle))) {
            e.preventDefault();
            return;
        }
        sortDrag = { moved: opts.group ? opts.group(item) : [item], opts: opts };
        if (e.dataTransfer) {
            e.dataTransfer.effectAllowed = 'move';
            // Some browsers start no drag without data.
            e.dataTransfer.setData('text/plain', '');
        }
        marked(true);
    });
    container.addEventListener('dragover', function (e) {
        if (!accepts()) {
            return;
        }
        e.preventDefault();
        var moved = sortDrag.moved;
        var item = itemOf(e.target);
        if (!item) {
            if (opts.into) {
                opts.into(moved);
            }
            return;
        }
        if (moved.indexOf(item) !== -1) {
            return;
        }
        var rect = item.getBoundingClientRect();
        var ratio = axis === 'x' ? (e.clientX - rect.left) / (rect.width || 1) : (e.clientY - rect.top) / (rect.height || 1);
        if (opts.over) {
            opts.over(moved, item, ratio);
            return;
        }
        var ref = ratio < 0.5 ? item : item.nextSibling;
        if (moved.indexOf(ref) !== -1) {
            return;
        }
        moved.forEach(function (m) { item.parentNode.insertBefore(m, ref); });
    });
    // Dropped here: not the browser's (opening the data as a link).
    container.addEventListener('drop', function (e) {
        if (accepts()) {
            e.preventDefault();
        }
    });
    container.addEventListener('dragend', function () {
        if (!sortDrag) {
            return;
        }
        var drag = sortDrag;
        marked(false);
        sortDrag = null;
        if (drag.opts.end) {
            drag.opts.end(drag.moved);
        }
    });
};
