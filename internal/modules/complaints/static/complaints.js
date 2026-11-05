// Complaints module (the complaints module), loaded after app.js
// (attached by its templates, {{attach}}).

// ===== Report a content item or a comment =====
// A Report link ([data-complain]) opens a window made of the page's
// #complaint-dialog: a reason, a note (needed for "Other"); Send posts
// them to /api/complaints and thanks the visitor. Without the template
// (the viewer may not report) the links go away; so do the comments'
// when comments take no complaints - on the page and in the comments
// loaded later (a behavior).
(function initComplaints() {
    const tpl = document.getElementById('complaint-dialog');
    const linkItem = 'li.comment-complain, .complaints-node-link';

    Kosatka.behaviors.complaints = {
        attach: function (context) {
            let selector = null;
            if (!tpl) {
                selector = '[data-complain]';
            } else if (tpl.dataset.comments !== '1') {
                selector = '[data-complain="comment"]';
            }
            if (!selector) {
                return;
            }
            Kosatka.once('complaints', selector, context).forEach(function (a) {
                (a.closest(linkItem) || a).remove();
            });
        }
    };
    if (!tpl) {
        return;
    }

    function open(link) {
        const kind = link.dataset.complain;
        const win = Kosatka.window({
            title: kind === 'node' ? tpl.dataset.titleNode : tpl.dataset.titleComment,
            className: 'complaints-window',
            opener: link
        });
        win.body.appendChild(tpl.content.cloneNode(true));
        const form = win.body.querySelector('form');
        const error = form.querySelector('.complaints-error');
        const note = form.querySelector('textarea[name="note"]');
        form.querySelector('.complaints-about').textContent = link.dataset.complainAbout || '';
        function fail(message) {
            error.textContent = message;
            error.hidden = false;
        }
        win.button(tpl.dataset.cancel).addEventListener('click', win.close);
        const send = win.button(tpl.dataset.send, true);
        function submit() {
            const checked = form.querySelector('input[name="reason"]:checked');
            error.hidden = true;
            if (!checked) {
                fail(tpl.dataset.needReason);
                return;
            }
            if (checked.value === tpl.dataset.other && !note.value.trim()) {
                fail(tpl.dataset.needNote);
                note.focus();
                return;
            }
            send.disabled = true;
            fetch('/api/complaints', {
                method: 'POST',
                credentials: 'same-origin',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({kind: kind, id: Number(link.dataset.complainId), reason: checked.value, note: note.value.trim()})
            })
                .then(function (res) {
                    return res.json().then(function (data) {
                        return {ok: res.ok, data: data};
                    });
                })
                .then(function (result) {
                    if (!result.ok) {
                        fail((result.data && result.data.error) || tpl.dataset.failed);
                        return;
                    }
                    win.close();
                    (link.closest(linkItem) || link).remove();
                    Kosatka.toast(result.data.message, 'success');
                })
                .catch(function () {
                    fail(tpl.dataset.failed);
                })
                .finally(function () {
                    send.disabled = false;
                });
        }
        send.addEventListener('click', submit);
        form.addEventListener('submit', function (e) {
            e.preventDefault();
            submit();
        });
        const first = form.querySelector('input[name="reason"]');
        if (first) {
            first.focus();
        }
    }

    document.addEventListener('click', function (e) {
        const link = e.target.closest('[data-complain]');
        if (!link) {
            return;
        }
        e.preventDefault();
        open(link);
    });
})();
