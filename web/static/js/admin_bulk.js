(function() {
    'use strict';

    // A bulk action's confirmation page (admin/bulk_confirm.html):
    // Confirm sends the action to data-api one item at a time, showing
    // the progress and the items' errors; when all went through, back to
    // data-redirect.
    function bulkConfirm(panel) {
        var action = panel.getAttribute('data-action');
        var apiUrl = panel.getAttribute('data-api');
        var redirect = panel.getAttribute('data-redirect');
        var ids = JSON.parse(panel.getAttribute('data-ids') || '[]');
        var total = ids.length;

        document.getElementById('bulkConfirmBtn').addEventListener('click', function() {
            var btn = this;
            var progressPanel = document.getElementById('bulkProgressPanel');
            var progressBar = document.getElementById('bulkProgressBar');
            var statusText = document.getElementById('bulkStatusText');

            btn.disabled = true;
            progressPanel.style.display = 'block';
            // An option changing the action (#bulkAltAction: spam and block).
            var alt = document.getElementById('bulkAltAction');
            if (alt && alt.checked) {
                action = alt.getAttribute('data-action');
            }

            var processed = 0;
            var index = 0;
            var errors = [];
            var errorsList = document.getElementById('bulkErrorsList');

            function processNext() {
                if (index >= ids.length) {
                    if (errors.length) {
                        statusText.textContent = Kosatka.t('@processed / @total - finished with errors', {'@processed': processed, '@total': total});
                        return;
                    }
                    window.location.href = redirect;
                    return;
                }
                var batch = ids.slice(index, index + 1);
                index += 1;

                fetch(apiUrl, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'same-origin',
                    body: JSON.stringify({ action: action, ids: batch })
                })
                .then(function(r) { return r.json(); })
                .then(function(res) {
                    if (res.pending) {
                        // The action isn't finished for this item yet (e.g. a user with a lot
                        // of content): repeat the same request.
                        index -= 1;
                        statusText.textContent = Kosatka.t('@processed / @total - deletion in progress…', {'@processed': processed, '@total': total});
                        processNext();
                        return;
                    }
                    // A refused request (no permission…) answers {"error": "…"}.
                    var failed = res.errors || (res.error ? [res.error] : []);
                    if (failed.length) {
                        errors = errors.concat(failed);
                        errorsList.innerHTML = '';
                        errors.forEach(function(e) {
                            var li = document.createElement('li');
                            li.textContent = e;
                            errorsList.appendChild(li);
                        });
                        errorsList.style.display = 'block';
                    }
                    processed += res.processed || 0;
                    var pct = Math.round((processed / total) * 100);
                    progressBar.style.width = pct + '%';
                    progressBar.setAttribute('aria-valuenow', pct);
                    progressBar.textContent = pct + '%';
                    statusText.textContent = processed + ' / ' + total;
                    processNext();
                })
                .catch(function(err) {
                    console.error('Bulk action error', err);
                    statusText.textContent = Kosatka.t('Error: @message', {'@message': err.message});
                    btn.disabled = false;
                });
            }
            processNext();
        });
    }

    document.addEventListener('DOMContentLoaded', function() {
        var panel = document.getElementById('bulkConfirmPanel');
        if (panel) bulkConfirm(panel);

        var bulkForm = document.getElementById('bulkForm');
        if (!bulkForm) return;

        var selectAll = document.getElementById('selectAll');
        var checkboxes = bulkForm.querySelectorAll('.bulk-checkbox');
        // The bar of the actions (data-bulk-bar): shown while rows are
        // checked, with their count (data-bulk-count).
        var bar = bulkForm.querySelector('[data-bulk-bar]');
        var count = bulkForm.querySelector('[data-bulk-count]');
        function syncBar() {
            if (!bar) return;
            var n = bulkForm.querySelectorAll('.bulk-checkbox:checked').length;
            bar.hidden = n === 0;
            if (count) count.textContent = n;
        }
        bulkForm.addEventListener('change', syncBar);
        syncBar();

        if (selectAll && checkboxes.length) {
            selectAll.addEventListener('change', function() {
                checkboxes.forEach(function(cb) {
                    cb.checked = selectAll.checked;
                });
            });

            checkboxes.forEach(function(cb) {
                cb.addEventListener('change', function() {
                    var allChecked = Array.from(checkboxes).every(function(c) { return c.checked; });
                    var anyChecked = Array.from(checkboxes).some(function(c) { return c.checked; });
                    selectAll.checked = allChecked;
                    selectAll.indeterminate = anyChecked && !allChecked;
                });
            });
        }

        bulkForm.addEventListener('submit', function(e) {
            // A row's own button (data-bulk-skip: it posts elsewhere,
            // formaction) needs no checked rows.
            if (e.submitter && e.submitter.hasAttribute('data-bulk-skip')) return;
            var checked = bulkForm.querySelectorAll('input[name="ids"]:checked');
            if (checked.length === 0) {
                e.preventDefault();
                Kosatka.alert(Kosatka.t('Select at least one item'));
                return false;
            }
        });
    });
})();
