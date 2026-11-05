// The ratings module: the node_rating block loads the widget with the
// visitor's own vote (the page may be cached) and sends the vote without
// leaving the page; without a script the block shows the total, and the
// form posts and comes back.
(function () {
    'use strict';

    function replaceWidget(block, html) {
        var holder = document.createElement('div');
        holder.innerHTML = html;
        var widget = holder.querySelector('.rating-widget');
        var old = block.querySelector('.rating-widget');
        if (widget && old) {
            old.replaceWith(widget);
        }
    }

    function load(block) {
        fetch(block.getAttribute('data-rating-endpoint'), {
            credentials: 'same-origin',
            headers: {'X-Requested-With': 'XMLHttpRequest'}
        }).then(function (resp) {
            return resp.ok ? resp.text() : '';
        }).then(function (html) {
            if (html) {
                replaceWidget(block, html);
            }
        }).catch(function () {});
    }

    function send(form, submitter) {
        var block = form.closest('.rating-block');
        var message = form.querySelector('.rating-message');
        var body = new FormData(form);
        if (submitter && submitter.name) {
            body.append(submitter.name, submitter.value);
        }
        form.querySelectorAll('button').forEach(function (b) { b.disabled = true; });
        fetch(form.action, {
            method: 'POST',
            credentials: 'same-origin',
            headers: {'X-Requested-With': 'XMLHttpRequest'},
            body: new URLSearchParams(body)
        }).then(function (resp) {
            return resp.json().then(function (data) { return {ok: resp.ok, data: data}; });
        }).then(function (res) {
            if (res.ok && res.data.html) {
                replaceWidget(block, res.data.html);
                return;
            }
            if (res.ok) {
                load(block);
                return;
            }
            form.querySelectorAll('button').forEach(function (b) { b.disabled = false; });
            if (message) {
                message.textContent = res.data.error || '';
            }
        }).catch(function () {
            form.querySelectorAll('button').forEach(function (b) { b.disabled = false; });
        });
    }

    document.addEventListener('submit', function (e) {
        var form = e.target.closest('.rating-block .rating-vote');
        if (!form) {
            return;
        }
        e.preventDefault();
        send(form, e.submitter);
    });

    // A behavior: the blocks of the page, and of the HTML put in it later
    // (a lazy block, a list's page) - each loaded once.
    window.Kosatka = window.Kosatka || {};
    Kosatka.behaviors = Kosatka.behaviors || {};
    Kosatka.behaviors.ratings = {
        attach: function (context) {
            Kosatka.once('rating-block', '.rating-block[data-rating-endpoint]', context).forEach(load);
        }
    };
})();
