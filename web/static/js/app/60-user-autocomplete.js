// ===== User autocomplete =====
// userAutocomplete - input[data-user-autocomplete] suggests users from
// /api/users/search once 3 characters are typed (Kosatka.autocomplete).
// data-user-id-input="<hidden input id>" - one user: the pick fills the
// input with the name and the hidden input with the id (a "change" event
// on it); emptying the input empties the id.
// data-user-list="<container id>" with data-user-field="<field name>" -
// several: each pick adds a tag with a remove button and a hidden input
// <field name>=<id> to the container; picked users aren't suggested again.
Kosatka.behaviors.userAutocomplete = {
    attach: function (context) {
        Kosatka.once('user-autocomplete', 'input[data-user-autocomplete]', context).forEach(function (input) {
            var idInput = document.getElementById(input.getAttribute('data-user-id-input') || '');
            var list = document.getElementById(input.getAttribute('data-user-list') || '');
            var field = input.getAttribute('data-user-field') || '';
            // Tags the page came with (a preselected user) remove like picked ones.
            if (list) {
                list.querySelectorAll('.user-autocomplete-tag button').forEach(function (b) {
                    b.addEventListener('click', function () { b.parentElement.remove(); });
                });
            }

            function picked(id) {
                return !!list && !!list.querySelector('input[type="hidden"][value="' + id + '"]');
            }

            function pick(user) {
                if (list) {
                    input.value = '';
                    if (picked(user.id)) return;
                    var tag = document.createElement('span');
                    tag.className = 'user-autocomplete-tag';
                    tag.appendChild(document.createTextNode(user.username + ' '));
                    var remove = document.createElement('button');
                    remove.type = 'button';
                    remove.textContent = '×';
                    remove.setAttribute('aria-label', Kosatka.t('Remove'));
                    remove.addEventListener('click', function () { tag.remove(); });
                    tag.appendChild(remove);
                    var hidden = document.createElement('input');
                    hidden.type = 'hidden';
                    hidden.name = field;
                    hidden.value = user.id;
                    tag.appendChild(hidden);
                    list.appendChild(tag);
                    return;
                }
                input.value = user.username;
                if (idInput) {
                    idInput.value = user.id;
                    idInput.dispatchEvent(new Event('change', { bubbles: true }));
                }
            }

            Kosatka.autocomplete(input, {
                minChars: 3,
                delay: 300,
                source: function (q) {
                    return fetch('/api/users/search?q=' + encodeURIComponent(q))
                        .then(function (r) { return r.json(); })
                        .then(function (data) {
                            return (data.users || []).filter(function (u) { return !picked(u.id); });
                        });
                },
                label: function (u) { return u.username + ' ' + Kosatka.t('(ID @id)', {'@id': u.id}); },
                pick: pick
            });
            // Emptied - no user.
            input.addEventListener('input', function () {
                if (idInput && input.value.trim() === '' && idInput.value !== '') {
                    idInput.value = '';
                    idInput.dispatchEvent(new Event('change', { bubbles: true }));
                }
            });
        });
    }
};
