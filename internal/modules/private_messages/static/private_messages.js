// Private messages module: the Messages tab of the profile (its
// conversations' list, private_messages_hooks.html), loaded after app.js
// on the pages that attach it ({{attach}}).

// ===== Conversations =====
// pmInbox - the list's bulk archive bar (#bulk-action-bar) shows while
// some conversation (.pm-checkbox) is checked, #select-all checks all;
// the participant filter's clear button (#filter-participant-clear) shows
// while a participant is picked (userAutocomplete fills
// #filter-participant) and clears it.
Kosatka.behaviors.pmInbox = {
    attach: function (context) {
        Kosatka.once('pm-inbox', '#bulk-action-bar', context).forEach(function (bar) {
            function updateBar() {
                bar.style.display = document.querySelector('.pm-checkbox:checked') ? 'block' : 'none';
            }
            var all = document.getElementById('select-all');
            if (all) {
                all.addEventListener('change', function () {
                    document.querySelectorAll('.pm-checkbox').forEach(function (cb) { cb.checked = all.checked; });
                    updateBar();
                });
            }
            document.querySelectorAll('.pm-checkbox').forEach(function (cb) {
                cb.addEventListener('change', updateBar);
            });
        });
        Kosatka.once('pm-participant', '#filter-participant-clear', context).forEach(function (clearBtn) {
            var hiddenInput = document.getElementById('filter-participant');
            if (!hiddenInput) return;
            hiddenInput.addEventListener('change', function () {
                clearBtn.style.visibility = hiddenInput.value ? 'visible' : 'hidden';
            });
            clearBtn.addEventListener('click', function () {
                document.getElementById('filter-participant-display').value = '';
                hiddenInput.value = '';
                hiddenInput.form.submit();
            });
        });
    }
};
