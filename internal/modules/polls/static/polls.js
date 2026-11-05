// Polls module: voting on a poll's page and in the "Poll" block, and the
// poll's options in its form, loaded after app.js (attached by
// poll/detail.html, node/edit_poll.html, {{attach}}, and by the block).

// ===== The "Poll" block =====
// A block placed with a poll (blocks/poll.html) is an empty
// [data-poll-embed] element: the poll is loaded from the module
// (/node/<id>/poll/embed), so a cached page still shows each visitor their
// vote; then pollVote votes in it.
Kosatka.behaviors.pollBlocks = {
    attach: function (context) {
        Kosatka.once('poll-embed', '[data-poll-embed]', context).forEach(function (block) {
            fetch(block.getAttribute('data-poll-embed'), { credentials: 'same-origin' })
                .then(function (resp) {
                    if (!resp.ok) {
                        throw new Error('HTTP ' + resp.status);
                    }
                    return resp.text();
                })
                .then(function (html) {
                    block.innerHTML = html;
                    Kosatka.attachBehaviors(block);
                })
                .catch(function () {
                    block.remove();
                });
        });
    }
};

// ===== Vote =====
// pollVote - a poll's vote form (form.poll-vote-form[data-poll-node-id]
// inside .poll-detail) votes through /api/node/<id>/poll/vote without
// leaving the page and updates the results (.poll-total-votes, the
// .poll-result-row of each option) and the checked options; an error goes
// to .poll-vote-msg. Without scripts the form posts as usual.
Kosatka.behaviors.pollVote = {
    attach: function (context) {
        Kosatka.once('poll-vote', 'form.poll-vote-form[data-poll-node-id]', context).forEach(function (form) {
            var poll = form.closest('.poll-detail') || document;
            var msg = poll.querySelector('.poll-vote-msg');
            var btn = form.querySelector('.poll-vote-btn');
            var loading = form.querySelector('.poll-vote-loading');

            function setLoading(on) {
                if (btn) btn.style.display = on ? 'none' : '';
                if (loading) loading.style.display = on ? 'block' : 'none';
                form.setAttribute('aria-busy', on ? 'true' : 'false');
                form.querySelectorAll('input[name="options"]').forEach(function (inp) {
                    inp.disabled = !!on;
                });
            }

            function showResults(data) {
                var total = poll.querySelector('.poll-total-votes');
                if (total && typeof data.total_votes === 'number') {
                    total.textContent = String(data.total_votes);
                }
                (data.options || []).forEach(function (opt) {
                    var row = poll.querySelector('.poll-result-row[data-poll-option-id="' + opt.id + '"]');
                    if (!row) return;
                    var percent = typeof opt.percent === 'number' ? opt.percent : 0;
                    var stats = row.querySelector('.poll-result-stats');
                    var fill = row.querySelector('.poll-result-bar-fill');
                    if (stats) stats.textContent = percent.toFixed(1) + '% (' + opt.count + ')';
                    if (fill) fill.style.width = percent + '%';
                });
                form.querySelectorAll('input[name="options"]').forEach(function (inp) {
                    var id = parseInt(inp.value, 10);
                    inp.checked = (data.options || []).some(function (o) { return o.id === id && o.selected; });
                });
            }

            form.addEventListener('submit', function (e) {
                e.preventDefault();
                if (msg) {
                    msg.style.display = 'none';
                    msg.textContent = '';
                }
                var options = [];
                form.querySelectorAll('input[name="options"]:checked').forEach(function (inp) {
                    var v = parseInt(inp.value, 10);
                    if (!isNaN(v) && v > 0) options.push(v);
                });
                setLoading(true);
                fetch('/api/node/' + encodeURIComponent(form.getAttribute('data-poll-node-id')) + '/poll/vote', {
                    method: 'POST',
                    credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
                    body: JSON.stringify({ options: options })
                }).then(function (res) {
                    return res.json().then(function (data) {
                        if (!res.ok) {
                            throw new Error((data && data.error) || Kosatka.t('Error @status', { '@status': res.status }));
                        }
                        return data;
                    });
                }).then(function (data) {
                    showResults(data);
                    if (data.show_form === false) {
                        form.style.display = 'none';
                    } else {
                        setLoading(false);
                    }
                }).catch(function (err) {
                    setLoading(false);
                    if (msg) {
                        msg.textContent = err.message || Kosatka.t('Could not submit your vote');
                        msg.style.display = 'block';
                    }
                });
            });
        });
    }
};

// ===== The poll's options (its form) =====
// pollOptions - #poll-options in node/edit_poll.html: "Add option"
// (#poll-add-option) adds a row from #poll-option-template; a row's
// Delete removes it while more than two are left.
Kosatka.behaviors.pollOptions = {
    attach: function (context) {
        Kosatka.once('poll-options', '#poll-options', context).forEach(function (container) {
            var addBtn = document.getElementById('poll-add-option');
            var tpl = document.getElementById('poll-option-template');
            if (!addBtn || !tpl) return;
            function refreshRemoveState() {
                var rows = container.querySelectorAll('.poll-option-row');
                rows.forEach(function (row) {
                    var btn = row.querySelector('.poll-remove-option');
                    if (btn) btn.disabled = rows.length <= 2;
                });
            }
            addBtn.addEventListener('click', function () {
                container.appendChild(tpl.content.cloneNode(true));
                refreshRemoveState();
            });
            container.addEventListener('click', function (e) {
                if (!e.target.classList.contains('poll-remove-option')) return;
                var row = e.target.closest('.poll-option-row');
                if (row && container.querySelectorAll('.poll-option-row').length > 2) {
                    row.remove();
                    refreshRemoveState();
                }
            });
            refreshRemoveState();
        });
    }
};
