// admin_blocks.js - the blocks page (admin/blocks/index.html): a block's
// row moves to another region by dragging it (Kosatka.sortable) or by its
// region select, within one by dragging; the order of all goes into the
// form's hidden input, saved with one button. A moved row is marked until
// then, and so is the page.
document.addEventListener('DOMContentLoaded', function() {
    var zones = document.querySelectorAll('tbody[data-region]');
    var form = document.getElementById('blocks-reorder-form');
    var orderInput = document.getElementById('blocks-order-input');
    if (!zones.length || !form || !orderInput) return;

    var unsaved = document.getElementById('blocks-unsaved');

    // markChanged - a moved row (another region or place) until the
    // order is saved, and the notice to save it.
    function markChanged(row) {
        row.classList.add('block-row-changed');
        if (unsaved) unsaved.hidden = false;
    }

    // syncEmpty - the "empty" row of a zone without blocks, none in one
    // with them.
    function syncEmpty(zone) {
        var empty = zone.querySelector('tr.block-zone-empty');
        var hasRows = zone.querySelector('tr[data-placement-id]');
        if (hasRows && empty) empty.remove();
        if (!hasRows && !empty) {
            var tr = document.createElement('tr');
            tr.className = 'block-zone-empty';
            var td = document.createElement('td');
            td.colSpan = 6;
            td.className = 'admin-table-empty';
            td.textContent = form.dataset.emptyLabel || '';
            tr.appendChild(td);
            zone.appendChild(tr);
        }
    }

    // A region chosen in a row's select moves the row there, to the end
    // (as dragging it does).
    form.addEventListener('change', function(e) {
        var select = e.target.closest('select[name^="region_"]');
        if (!select) return;
        var row = select.closest('tr[data-placement-id]');
        var from = row.closest('tbody[data-region]');
        var to = form.querySelector('tbody[data-region="' + select.value + '"]');
        if (!to || to === from) return;
        to.appendChild(row);
        syncEmpty(from);
        syncEmpty(to);
        markChanged(row);
        orderInput.value = currentOrder();
        row.scrollIntoView({block: 'nearest', behavior: 'smooth'});
    });

    function currentOrder() {
        var ids = [];
        zones.forEach(function(zone) {
            zone.querySelectorAll('tr[data-placement-id]').forEach(function(row) {
                ids.push(row.dataset.placementId);
            });
        });
        return ids.join(',');
    }

    function clearDragOver() {
        document.querySelectorAll('.block-zone.drag-over').forEach(function(z) {
            z.classList.remove('drag-over');
        });
    }

    // A row dragged (Kosatka.sortable) within its region or into another
    // one (the end of an empty one); its select follows.
    zones.forEach(function(zone) {
        Kosatka.sortable(zone, {
            items: 'tr[data-placement-id]',
            shared: 'block-regions',
            into: function(moved) {
                var empty = zone.querySelector('tr.block-zone-empty');
                if (empty) empty.remove();
                zone.appendChild(moved[0]);
            },
            end: function(moved) {
                var row = moved[0];
                clearDragOver();
                var newRegion = row.closest('tbody[data-region]').dataset.region;
                var select = row.querySelector('select[name^="region_"]');
                if (select && select.value !== newRegion) {
                    select.value = newRegion;
                    select.dispatchEvent(new Event('change', { bubbles: true }));
                }
                zones.forEach(syncEmpty);
                markChanged(row);
                orderInput.value = currentOrder();
            }
        });
        // The region under the dragged row stands out.
        zone.addEventListener('dragover', function() {
            if (!form.querySelector('tr[data-placement-id].dragging')) return;
            clearDragOver();
            zone.closest('.block-zone').classList.add('drag-over');
        });
    });

    form.addEventListener('submit', function() {
        orderInput.value = currentOrder();
    });
});
