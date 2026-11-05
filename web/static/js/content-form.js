// content-form.js - the content form's own scripts (node/edit_scripts.html,
// after Tagify): the pickers (tagPickers, a behavior: a block's form put in
// the page later gets them too) and the address that follows the type's
// pattern.

// tagPickers - the pickers of the content form (a block's too): a Tagify
// widget over an input with data-tag-picker - "terms" (a multi-value
// taxonomy field, free tagging in data-vocabulary-id: new ones as
// "new:<name>"), "nodes" (a reference field, /api/nodes/search of
// data-types) or "users" (a user field, /api/users/search of
// data-roles); data-single - a new pick replaces the current one. The
// hidden input data-tag-target (its id) holds the IDs chosen, comma-
// separated in order; data-tags - the chosen ones' tags ([{id, value}]).
Kosatka.behaviors.tagPickers = {
    attach: function (context) {
        Kosatka.once('tag-picker', '[data-tag-picker]', context).forEach(function (displayInput) {
            var hiddenInput = document.getElementById(displayInput.getAttribute('data-tag-target'));
            if (!hiddenInput || typeof Tagify === 'undefined') return;
            var kind = displayInput.getAttribute('data-tag-picker');
            var single = displayInput.hasAttribute('data-single');
            var existing = [];
            try { existing = JSON.parse(displayInput.getAttribute('data-tags') || '[]'); } catch (e) { existing = []; }
            var terms = kind === 'terms';
            var fieldTagify = new Tagify(displayInput, {
                tagTextProp: 'value',
                enforceWhitelist: !terms,
                dropdown: { enabled: 1, maxItems: terms ? 20 : 15, closeOnSelect: true, highlightFirst: true },
                whitelist: existing
            });
            function updateField() {
                hiddenInput.value = fieldTagify.value.map(function (t) {
                    if (t.id && t.id > 0) return String(t.id);
                    return terms ? 'new:' + t.value : '';
                }).filter(Boolean).join(',');
            }
            function searchURL(value) {
                if (terms) {
                    return '/api/taxonomy/autocomplete?vid=' + encodeURIComponent(displayInput.getAttribute('data-vocabulary-id')) + '&q=' + encodeURIComponent(value);
                }
                if (kind === 'users') {
                    var roles = displayInput.getAttribute('data-roles');
                    return '/api/users/search?q=' + encodeURIComponent(value) + (roles ? '&roles=' + roles : '');
                }
                var types = displayInput.getAttribute('data-types');
                return '/api/nodes/search?q=' + encodeURIComponent(value) + (types ? '&types=' + encodeURIComponent(types) : '');
            }
            function suggestions(data) {
                if (terms) return (data.suggestions || []).map(function (item) { return {id: item.id, value: item.name}; });
                if (kind === 'users') return (data.users || []).map(function (u) { return {id: u.id, value: u.username}; });
                return (data.nodes || []).map(function (n) { return {id: n.id, value: n.match ? n.title + ' - ' + n.match : n.title}; });
            }
            var searchTimeout;
            fieldTagify.on('input', function (e) {
                var value = e.detail.value;
                if (!value || value.length < 2) return;
                clearTimeout(searchTimeout);
                searchTimeout = setTimeout(function () {
                    fetch(searchURL(value), {credentials: 'same-origin'})
                        .then(function (r) { return r.json(); })
                        .then(function (data) {
                            fieldTagify.whitelist = suggestions(data);
                            fieldTagify.dropdown.show(value);
                        })
                        .catch(function (err) { console.error('Error searching for ' + kind + ':', err); });
                }, terms ? 0 : 300);
            });
            if (existing.length > 0) {
                fieldTagify.addTags(existing);
            }
            fieldTagify.on('add', function (e) {
                if (single) {
                    fieldTagify.getTagElms().forEach(function (el) {
                        if (el !== e.detail.tag) fieldTagify.removeTags(el, true);
                    });
                }
                updateField();
            });
            fieldTagify.on('remove', updateField);
            // The terms' input starts empty; the others' keep the IDs sent
            // (one whose content is gone among them).
            if (terms) updateField();
        });
    }
};

document.addEventListener('DOMContentLoaded', function() {
    // The URL alias follows the type's pattern while "generate
    // automatically" is on (edit_node_settings.html): its input is off
    // then, so it isn't sent.
    var pathAuto = document.getElementById('path_auto');
    var slugInput = document.getElementById('slug');
    if (pathAuto && slugInput) {
        var syncSlug = function () { slugInput.disabled = pathAuto.checked; };
        pathAuto.addEventListener('change', syncSlug);
        syncSlug();
    }
});
