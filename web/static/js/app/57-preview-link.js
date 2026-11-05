// ===== Preview links =====
// The content form's "Link for viewing" card ([data-preview-link] - the
// node's /preview-link address): "Make a link" asks for a new one
// (an earlier one stops working) and shows it to copy, once - the site
// keeps only its hash; "Revoke" ends it.
Kosatka.behaviors.previewLink = {
    attach: function (context) {
        Kosatka.once('preview-link', '[data-preview-link]', context).forEach(function (card) {
            const address = card.getAttribute('data-preview-link');
            const state = card.querySelector('[data-preview-link-state]');
            const urlBox = card.querySelector('[data-preview-link-url]');
            const input = urlBox.querySelector('input');
            const create = card.querySelector('[data-preview-link-create]');
            const revoke = card.querySelector('[data-preview-link-revoke]');
            const token = document.querySelector('meta[name="csrf-token"]');
            function post(action) {
                const body = new URLSearchParams({action: action});
                return fetch(address, {
                    method: 'POST',
                    credentials: 'same-origin',
                    headers: {'X-CSRF-Token': token ? token.content : '', 'Content-Type': 'application/x-www-form-urlencoded'},
                    body: body
                }).then(function (r) {
                    return r.json().then(function (data) {
                        if (!r.ok) {
                            throw new Error(data.error || Kosatka.t('Error'));
                        }
                        return data;
                    });
                });
            }
            create.addEventListener('click', function () {
                post('create').then(function (data) {
                    input.value = data.url;
                    urlBox.hidden = false;
                    const until = new Date(data.expires).toLocaleString(document.documentElement.lang || undefined);
                    state.textContent = Kosatka.t('A link works until @date.', {'@date': until}) + ' ' + Kosatka.t('Copy it now: it is shown only once.');
                    create.textContent = Kosatka.t('Make a new link');
                    revoke.hidden = false;
                    input.select();
                }).catch(function (err) { Kosatka.alert(err.message); });
            });
            revoke.addEventListener('click', function () {
                post('revoke').then(function () {
                    input.value = '';
                    urlBox.hidden = true;
                    state.textContent = Kosatka.t('The link is revoked: it shows nothing now.');
                    create.textContent = Kosatka.t('Make a link');
                    revoke.hidden = true;
                }).catch(function (err) { Kosatka.alert(err.message); });
            });
            card.querySelector('[data-preview-link-copy]').addEventListener('click', function () {
                input.select();
                if (navigator.clipboard) {
                    navigator.clipboard.writeText(input.value);
                }
            });
        });
    }
};
