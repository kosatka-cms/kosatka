// admin_views.js - client-side logic for admin/views/page.html, the one admin
// page that uses AJAX modals instead of one page per action. Uses the fetch/JSON pattern (CSRF header, JSON errors) and
// Bootstrap modals populated from data attributes; never the browser's
// confirm/alert (Kosatka.confirm, Kosatka.alert).
(function () {
    'use strict';

    var VIEW_ID = (location.pathname.match(/\/admin\/config\/views\/(\d+)/) || [])[1];

    // The editors' dialogs (the header and footer texts) render outside
    // #settingsModal, and Bootstrap's modal focus trap steals focus back
    // from them: this listener, on the document before Bootstrap's (added
    // when a modal opens), stops focusin inside them from reaching it.
    document.addEventListener('focusin', function (e) {
        if (e.target.closest('.tox-tinymce-aux, .tox-dialog-wrap, .tox-dialog')) {
            e.stopImmediatePropagation();
        }
    });

    async function postForm(url, params) {
        var resp = await fetch(url, {
            method: 'POST',
            headers: {
                'X-Requested-With': 'XMLHttpRequest',
                'Content-Type': 'application/x-www-form-urlencoded'
            },
            body: params.toString()
        });
        var data = {};
        try { data = await resp.json(); } catch (e) { /* empty body is fine */ }
        if (!resp.ok) throw new Error(data.error || Kosatka.t('Error'));
        return data;
    }

    async function postJSON(url, obj) {
        var resp = await fetch(url, {
            method: 'POST',
            headers: {
                'X-Requested-With': 'XMLHttpRequest',
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(obj)
        });
        var data = {};
        try { data = await resp.json(); } catch (e) { /* empty body is fine */ }
        if (!resp.ok) throw new Error(data.error || Kosatka.t('Error'));
        return data;
    }

    async function postDelete(url) {
        var resp = await fetch(url, {
            method: 'POST',
            headers: { 'X-Requested-With': 'XMLHttpRequest' }
        });
        var data = {};
        try { data = await resp.json(); } catch (e) { /* empty body is fine */ }
        if (!resp.ok) throw new Error(data.error || Kosatka.t('Error'));
        return data;
    }

    function formToParams(form) {
        var params = new URLSearchParams();
        new FormData(form).forEach(function (value, key) { params.append(key, value); });
        return params;
    }

    // setupFieldCombobox turns a target <select> (filter-target /
    // field-storage-id) into a single type-to-filter field. The select stays the
    // real form control (hidden with d-none in the modal's template); a text <input> and
    // a Bootstrap .dropdown-menu are built from its <option>/<optgroup> markup,
    // keeping the groups as dropdown-header rows.
    // The settings of a module's sort criterion or field
    // (view_module_params): only the chosen target's group shows and is
    // sent; params - an edited row's values (else the defaults stay).
    function showModuleParams(form, target, params) {
        form.querySelectorAll('.view-module-params').forEach(function (group) {
            var on = group.getAttribute('data-module-target') === target;
            group.classList.toggle('d-none', !on);
            group.querySelectorAll('[data-param]').forEach(function (input) {
                input.disabled = !on;
                var key = input.getAttribute('data-param');
                if (on && params && params[key] !== undefined) input.value = params[key];
            });
        });
    }

    // rowParams - a list row's module params (its data-params JSON), null
    // when it has none.
    function rowParams(row) {
        try {
            return JSON.parse(row.getAttribute('data-params') || 'null');
        } catch (e) {
            return null;
        }
    }

    // setupFieldCombobox - a searchable field picker (Kosatka.autocomplete)
    // over a hidden select, the source of truth (its optgroups - the
    // headings): every field once the input gets the focus, fewer as one
    // types; a pick sets the select (a "change" event).
    function setupFieldCombobox(input, select) {
        Kosatka.autocomplete(input, {
            minChars: 0,
            delay: 0,
            onFocus: true,
            empty: Kosatka.t('Nothing found'),
            source: function (q) {
                q = q.toLowerCase();
                var items = [];
                Array.prototype.forEach.call(select.querySelectorAll('option'), function (opt) {
                    if (!opt.value || (q && opt.textContent.toLowerCase().indexOf(q) === -1)) return;
                    var group = opt.parentNode.tagName === 'OPTGROUP' ? opt.parentNode.label : '';
                    items.push({ value: opt.value, text: opt.textContent, group: group });
                });
                return items;
            },
            label: function (it) { return it.text; },
            group: function (it) { return it.group; },
            pick: function (it) {
                select.value = it.value;
                select.dispatchEvent(new Event('change', { bubbles: true }));
                input.value = it.text;
            }
        });
    }

    // syncComboboxDisplay shows the selected option's text in the combobox
    // input; call it after setting select.value programmatically. The select,
    // not the input, is the source of truth.
    function syncComboboxDisplay(input, select) {
        var opt = select.selectedOptions[0];
        input.value = opt && opt.value ? opt.textContent : '';
    }

    document.addEventListener('DOMContentLoaded', function () {
        // ===== SEO and grid-columns sections apply only to the page display and
        // the grid style respectively: show/hide them accordingly.
        var seoSection = document.getElementById('settings-seo-section');
        var gridColumnsSection = document.getElementById('settings-grid-columns-section');
        var carouselSection = document.getElementById('settings-carousel-section');
        var urlPathSection = document.getElementById('settings-url-path-section');
        var attachSection = document.getElementById('settings-attach-section');
        var termSection = document.getElementById('settings-term-section');
        var userTabSection = document.getElementById('settings-user-tab-section');
        var settingsForm = document.getElementById('settingsForm');

        function toggleSeoAndGridColumns() {
            if (!settingsForm) return;
            var displayType = settingsForm.querySelector('input[name="display_type"]:checked');
            var displayStyle = settingsForm.querySelector('input[name="display_style"]:checked');
            var isPage = !displayType || displayType.value === 'page';
            if (seoSection) seoSection.classList.toggle('d-none', !isPage);
            if (urlPathSection) urlPathSection.classList.toggle('d-none', !isPage);
            if (attachSection) attachSection.classList.toggle('d-none', !displayType || displayType.value !== 'attachment');
            if (termSection) termSection.classList.toggle('d-none', !displayType || displayType.value !== 'term');
            if (userTabSection) userTabSection.classList.toggle('d-none', !displayType || displayType.value !== 'user_tab');
            if (gridColumnsSection) gridColumnsSection.classList.toggle('d-none', !displayStyle || displayStyle.value !== 'grid');
            if (carouselSection) carouselSection.classList.toggle('d-none', !displayStyle || displayStyle.value !== 'carousel');
            togglePaginationAjaxSection();
        }

        // pagination_ajax only applies when show_pagination is on, and not
        // to a block: its pages always load in place (its note says so).
        var paginationAjaxSection = document.getElementById('settings-pagination-ajax-section');
        var paginationBlockNote = document.getElementById('settings-pagination-block-note');
        var showPaginationCheckbox = document.getElementById('settings-show-pagination');
        function togglePaginationAjaxSection() {
            var displayType = settingsForm ? settingsForm.querySelector('input[name="display_type"]:checked') : null;
            var isBlock = !!displayType && displayType.value === 'block';
            if (paginationAjaxSection && showPaginationCheckbox) {
                paginationAjaxSection.classList.toggle('d-none', !showPaginationCheckbox.checked || isBlock);
            }
            if (paginationBlockNote) paginationBlockNote.classList.toggle('d-none', !isBlock);
        }

        // The glossary_field_storage_id picker only applies when show_glossary is on.
        var glossaryFieldSection = document.getElementById('settings-glossary-field-section');
        var showGlossaryCheckbox = document.getElementById('settings-show-glossary');
        function toggleGlossaryFieldSection() {
            if (glossaryFieldSection && showGlossaryCheckbox) {
                glossaryFieldSection.classList.toggle('d-none', !showGlossaryCheckbox.checked);
            }
        }

        // glossary-field-storage-id is a single always-rendered Settings field (no
        // add/edit modal), so it is wired up once here; syncComboboxDisplay shows
        // the server-rendered selected option.
        var glossaryFieldSelect = document.getElementById('glossary-field-storage-id');
        var glossaryFieldSearch = document.getElementById('glossary-field-storage-search');
        if (glossaryFieldSelect) {
            setupFieldCombobox(glossaryFieldSearch, glossaryFieldSelect);
            syncComboboxDisplay(glossaryFieldSearch, glossaryFieldSelect);
        }

        if (settingsForm) {
            settingsForm.addEventListener('change', function (e) {
                if (e.target.name === 'display_type' || e.target.name === 'display_style') toggleSeoAndGridColumns();
                if (e.target.name === 'show_pagination') togglePaginationAjaxSection();
                if (e.target.name === 'show_glossary') toggleGlossaryFieldSection();
            });
            toggleSeoAndGridColumns();
            togglePaginationAjaxSection();
            toggleGlossaryFieldSection();
        }

        // ===== Publish status refresh. Settings/Filters/Sorts/Fields each save
        // over their own AJAX endpoint and any of them can change whether there are
        // unpublished changes, so every success handler calls refreshPublishStatus()
        // to update the reset button and the tab's "*" indicator without a reload. =====
        function setTabUnpublished(hasChanges) {
            var tab = document.querySelector('#view-tab-strip a[data-tab-id="' + VIEW_ID + '"]');
            if (!tab) return;
            var badge = tab.querySelector('.unpublished-badge');
            if (hasChanges && !badge) {
                badge = document.createElement('span');
                badge.className = 'unpublished-badge text-warning fw-bold';
                badge.title = Kosatka.t('Unpublished changes');
                badge.textContent = ' *';
                tab.appendChild(badge);
            } else if (!hasChanges && badge) {
                badge.remove();
            }
        }
        async function refreshPublishStatus() {
            var statusEl = document.getElementById('view-publish-status');
            if (!statusEl) return;
            try {
                var resp = await fetch('/admin/config/views/' + VIEW_ID + '/publish-status', {
                    headers: { 'X-Requested-With': 'XMLHttpRequest' }
                });
                var data = await resp.json();
                statusEl.innerHTML = data.html || '';
                var discardForm = statusEl.querySelector('form[action$="/discard"]');
                setTabUnpublished(!!discardForm && !discardForm.classList.contains('d-none'));
                // Something of the display changed: the preview follows.
                document.dispatchEvent(new CustomEvent('kosatka:view-changed'));
            } catch (err) {
                // Non-fatal - the status/badge just won't refresh this time.
            }
        }

        // ===== Settings modal save =====
        if (settingsForm) {
            settingsForm.addEventListener('submit', async function (e) {
                e.preventDefault();
                // header_text/footer_text are TinyMCE editors: triggerSave() copies their
                // live content into the underlying <textarea>s before formToParams reads
                // the form.
                if (window.tinymce) tinymce.triggerSave();
                try {
                    var data = await postForm(settingsForm.action, formToParams(settingsForm));
                    document.getElementById('settings-summary').innerHTML = data.html || '';
                    // Update the display's tab label from the submitted field (the response
                    // only carries the settings summary). No-op for Default (no matching
                    // data-tab-id) and for a lone display (no tab strip).
                    var labelInput = document.getElementById('settings-label');
                    var tab = labelInput && document.querySelector('#view-tab-strip a[data-tab-id="' + VIEW_ID + '"]');
                    if (tab) {
                        // Keep the unpublished-changes badge (a nested <span>, see
                        // setTabUnpublished) that a textContent write would wipe.
                        var badge = tab.querySelector('.unpublished-badge');
                        tab.textContent = labelInput.value;
                        if (badge) tab.appendChild(badge);
                    }
                    bootstrap.Modal.getInstance(document.getElementById('settingsModal')).hide();
                    refreshPublishStatus();
                } catch (err) {
                    Kosatka.alert(Kosatka.t('Error saving settings: @error', {'@error': err.message}));
                }
            });
        }

        // ===== Publish changes. The reset button is a plain form POST, not wired
        // here: it rewrites Settings, Filters, Sorts and Fields at once, so a full
        // reload is simpler. =====
        var publishForm = document.getElementById('view-publish-form');
        if (publishForm) {
            publishForm.addEventListener('submit', async function (e) {
                e.preventDefault();
                try {
                    await postForm(publishForm.action, new URLSearchParams());
                    await refreshPublishStatus();
                } catch (err) {
                    Kosatka.alert(Kosatka.t('Publishing error: @error', {'@error': err.message}));
                }
            });
        }

        // confirmDelete - action once the site's dialog gets a yes (the
        // filters', fields' and sorts' Delete; without JS their own
        // confirmation pages).
        function confirmDelete(message, action) {
            Kosatka.confirm(message, Kosatka.t('Yes, delete')).then(function (yes) {
                if (yes) action();
            });
        }

        // ===== Reordering (Kosatka.sortable by the rows' .drag-handle),
        // saved over AJAX as soon as dropped: this page never reloads.
        function setupReorder(containerId, rowSelector, idAttr, saveUrl, onSaved) {
            var container = document.getElementById(containerId);
            if (!container) return;
            Kosatka.sortable(container, {
                items: rowSelector,
                handle: '.drag-handle',
                end: async function () {
                    var rows = Array.prototype.slice.call(container.querySelectorAll(rowSelector));
                    var order = rows.map(function (row) { return parseInt(row.getAttribute(idAttr), 10); });
                    try {
                        var data = await postJSON(saveUrl, { order: order });
                        if (onSaved && data.html) onSaved(data.html);
                        refreshPublishStatus();
                    } catch (err) {
                        Kosatka.alert(Kosatka.t('Error saving the order: @error', {'@error': err.message}));
                    }
                }
            });
        }

        // ===== Filters card =====
        var filterModalEl = document.getElementById('filterModal');
        var filterForm = document.getElementById('filterForm');
        var filterEditId = null;
        var filterTargetSelect = document.getElementById('filter-target');
        var filterTargetSearch = document.getElementById('filter-target-search');
        setupFieldCombobox(filterTargetSearch, filterTargetSelect);
        var filterOperatorSelect = document.getElementById('filter-operator');
        var filterValueInput = document.getElementById('filter-value');
        var filterValueTextWrap = document.getElementById('filter-value-text-wrap');
        var filterValueTermsWrap = document.getElementById('filter-value-terms-wrap');
        var filterValueTermsSelect = document.getElementById('filter-value-terms');
        var taxonomyTermsCatalog = document.getElementById('taxonomy-terms-catalog');
        var filterValueContentTypesWrap = document.getElementById('filter-value-content-types-wrap');
        var filterValueContentTypesSelect = document.getElementById('filter-value-content-types');
        var filterValueModuleOptionsWrap = document.getElementById('filter-value-module-options-wrap');
        var filterValueModuleOptionsSelect = document.getElementById('filter-value-module-options');
        var filterValueVocabularyWrap = document.getElementById('filter-value-vocabulary-wrap');
        var filterValueVocabularySelect = document.getElementById('filter-value-vocabulary');
        var filterValueRelativeWrap = document.getElementById('filter-value-relative-wrap');
        var filterValueRelativeCheckbox = document.getElementById('filter-value-relative');
        var filterContextualWrap = document.getElementById('filter-contextual-wrap');
        var filterContextualCheckbox = document.getElementById('filter-contextual');
        var filterExposedCheckbox = document.getElementById('filter-exposed');

        // Narrow the operator list to what the chosen target supports (e.g. image
        // fields only get empty/not_empty); data-operators mirrors the server's
        // validOperatorsFor/validOperatorsForBuiltin.
        function updateFilterOperatorOptions() {
            var opt = filterTargetSelect.selectedOptions[0];
            var allowed = (opt ? opt.getAttribute('data-operators') : '') || '';
            allowed = allowed.split(' ').filter(Boolean);
            var options = Array.prototype.slice.call(filterOperatorSelect.options);
            var stillValid = false;
            options.forEach(function (o) {
                var ok = allowed.indexOf(o.value) !== -1;
                o.hidden = !ok;
                if (ok && o.selected) stillValid = true;
            });
            if (!stillValid) {
                var firstAllowed = options.filter(function (o) { return !o.hidden; })[0];
                if (firstAllowed) filterOperatorSelect.value = firstAllowed.value;
            }
        }

        // Show the value widget matching the chosen target and hide the others:
        // a term picker for taxonomy fields, a content-type picker for the
        // content_type builtin, a vocabulary picker for the builtins comparing
        // the current content's terms, a free-text input otherwise.
        // preselectValue is the row's current comma-separated value (term IDs,
        // machine names or a vocabulary ID), empty in add mode.
        function updateFilterValueWidget(preselectValue) {
            var target = filterTargetSelect.value;
            var opt = filterTargetSelect.selectedOptions[0];
            var fieldType = opt ? opt.getAttribute('data-field-type') : '';
            var selected = (preselectValue || '').split(',').map(function (s) { return s.trim(); }).filter(Boolean);

            filterValueTextWrap.classList.add('d-none');
            filterValueTermsWrap.classList.add('d-none');
            filterValueContentTypesWrap.classList.add('d-none');
            filterValueModuleOptionsWrap.classList.add('d-none');
            filterValueVocabularyWrap.classList.add('d-none');
            filterValueRelativeWrap.classList.add('d-none');
            filterValueRelativeCheckbox.checked = false;

            // Contextual applies only to reference fields and the builtins that
            // read the current content (entity.ViewFilterDef.Contextual); hidden
            // and unchecked otherwise. The last three builtins are contextual
            // only (views.validateFilter), so it is checked for them.
            var contextualOnly = target === 'builtin:similar_to_current_node' || target === 'builtin:shares_terms_with_current' || target === 'builtin:has_current_term';
            if (contextualOnly || target === 'builtin:created_by' || target === 'builtin:comment_created_by' || fieldType === 'reference') {
                filterContextualWrap.classList.remove('d-none');
                if (contextualOnly) filterContextualCheckbox.checked = true;
            } else {
                filterContextualWrap.classList.add('d-none');
                filterContextualCheckbox.checked = false;
            }

            if (fieldType === 'taxonomy') {
                // Term picker populated from the matching hidden <select> in
                // #taxonomy-terms-catalog, without another AJAX request.
                // "field:<id>" or "block:<blocks field>:<id>".
                var fieldId = target.split(':').pop();
                var catalogSelect = taxonomyTermsCatalog.querySelector('select[data-field-id="' + fieldId + '"]');
                filterValueTermsSelect.innerHTML = '';
                if (catalogSelect) {
                    Array.prototype.forEach.call(catalogSelect.options, function (o) {
                        var clone = o.cloneNode(true);
                        clone.selected = selected.indexOf(o.value) !== -1;
                        filterValueTermsSelect.appendChild(clone);
                    });
                }
                filterValueTermsWrap.classList.remove('d-none');
                return;
            }

            // A module's choice filter: its values (data-options) to pick.
            var moduleOptions = opt ? opt.getAttribute('data-options') : '';
            if (moduleOptions) {
                filterValueModuleOptionsSelect.innerHTML = '';
                JSON.parse(moduleOptions).forEach(function (o) {
                    var option = document.createElement('option');
                    option.value = o.value;
                    option.textContent = o.label;
                    option.selected = selected.indexOf(o.value) !== -1;
                    filterValueModuleOptionsSelect.appendChild(option);
                });
                filterValueModuleOptionsWrap.classList.remove('d-none');
                return;
            }

            if (target === 'builtin:content_type') {
                Array.prototype.forEach.call(filterValueContentTypesSelect.options, function (o) {
                    o.selected = selected.indexOf(o.value) !== -1;
                });
                filterValueContentTypesWrap.classList.remove('d-none');
                return;
            }

            // The value of these two is an optional vocabulary ID (empty -
            // every vocabulary).
            if (target === 'builtin:similar_to_current_node' || target === 'builtin:shares_terms_with_current') {
                filterValueVocabularySelect.value = selected[0] || '';
                // A vocabulary deleted since: keep the stored ID rather than
                // silently widening the filter to every vocabulary.
                if (filterValueVocabularySelect.value !== (selected[0] || '')) {
                    var missing = document.createElement('option');
                    missing.value = selected[0];
                    missing.textContent = selected[0];
                    filterValueVocabularySelect.appendChild(missing);
                    filterValueVocabularySelect.value = selected[0];
                }
                filterValueVocabularyWrap.classList.remove('d-none');
                return;
            }

            filterValueTextWrap.classList.remove('d-none');
            if (fieldType === 'date') {
                filterValueRelativeWrap.classList.remove('d-none');
            }
        }

        filterValueTermsSelect.addEventListener('change', function () {
            var ids = Array.prototype.filter.call(filterValueTermsSelect.options, function (o) { return o.selected; })
                .map(function (o) { return o.value; });
            filterValueInput.value = ids.join(',');
        });

        filterValueContentTypesSelect.addEventListener('change', function () {
            var names = Array.prototype.filter.call(filterValueContentTypesSelect.options, function (o) { return o.selected; })
                .map(function (o) { return o.value; });
            filterValueInput.value = names.join(',');
        });

        filterValueModuleOptionsSelect.addEventListener('change', function () {
            var values = Array.prototype.filter.call(filterValueModuleOptionsSelect.options, function (o) { return o.selected; })
                .map(function (o) { return o.value; });
            filterValueInput.value = values.join(',');
        });

        filterValueVocabularySelect.addEventListener('change', function () {
            filterValueInput.value = filterValueVocabularySelect.value;
        });

        filterTargetSelect.addEventListener('change', function () {
            updateFilterOperatorOptions();
            updateFilterValueWidget('');
            filterValueInput.value = '';
        });

        // Contextual and Exposed are mutually exclusive (see
        // entity.ViewFilterDef.Contextual); validateFilter enforces it server-side,
        // this just prevents submitting the conflict.
        filterContextualCheckbox.addEventListener('change', function () {
            if (filterContextualCheckbox.checked) filterExposedCheckbox.checked = false;
        });
        filterExposedCheckbox.addEventListener('change', function () {
            if (filterExposedCheckbox.checked) filterContextualCheckbox.checked = false;
        });

        function openFilterModal(row) {
            if (row) {
                filterEditId = row.getAttribute('data-filter-id');
                document.getElementById('filterModalTitle').textContent = Kosatka.t('Edit filter');
                filterTargetSelect.value = row.getAttribute('data-target') || '';
                syncComboboxDisplay(filterTargetSearch, filterTargetSelect);
                var value = row.getAttribute('data-value') || '';
                filterValueInput.value = value;
                updateFilterOperatorOptions();
                filterOperatorSelect.value = row.getAttribute('data-operator') || '';
                updateFilterValueWidget(value);
                filterValueRelativeCheckbox.checked = row.getAttribute('data-value-relative') === '1';
                document.getElementById('filter-exposed').checked = row.getAttribute('data-exposed') === '1';
                document.getElementById('filter-exposed-param').value = row.getAttribute('data-exposed-param') || '';
                document.getElementById('filter-exposed-label').value = row.getAttribute('data-exposed-label') || '';
                // Must run after updateFilterValueWidget, which resets
                // filterContextualCheckbox for non-contextual targets.
                filterContextualCheckbox.checked = row.getAttribute('data-contextual') === '1';
            } else {
                filterEditId = null;
                document.getElementById('filterModalTitle').textContent = Kosatka.t('Add filter');
                filterForm.reset();
                syncComboboxDisplay(filterTargetSearch, filterTargetSelect);
                updateFilterOperatorOptions();
                updateFilterValueWidget('');
            }
            new bootstrap.Modal(filterModalEl).show();
        }

        var filterAddBtn = document.getElementById('filter-add-btn');
        if (filterAddBtn) filterAddBtn.addEventListener('click', function () { openFilterModal(null); });

        var filtersList = document.getElementById('filters-list');
        if (filtersList) {
            filtersList.addEventListener('click', function (e) {
                var editBtn = e.target.closest('.filter-edit-btn');
                if (editBtn) { openFilterModal(editBtn.closest('tr[data-filter-id]')); return; }
                var delBtn = e.target.closest('.filter-delete-btn');
                if (delBtn) {
                    var row = delBtn.closest('tr[data-filter-id]');
                    var filterId = row.getAttribute('data-filter-id');
                    confirmDelete(Kosatka.t('Delete this filter?'), function () {
                        return postDelete('/admin/config/views/' + VIEW_ID + '/filters/' + filterId + '/delete')
                            .then(function (data) { filtersList.innerHTML = data.html || ''; refreshPublishStatus(); })
                            .catch(function (err) { Kosatka.alert(Kosatka.t('Error deleting the filter: @error', {'@error': err.message})); });
                    });
                }
            });
            setupReorder('filters-list', 'tr[data-filter-id]', 'data-filter-id',
                '/admin/config/views/' + VIEW_ID + '/filters/reorder',
                function (html) { filtersList.innerHTML = html; });
        }

        if (filterForm) {
            filterForm.addEventListener('submit', async function (e) {
                e.preventDefault();
                // filter-target is hidden (see setupFieldCombobox) and may be blank, so it
                // is checked here: a hidden "required" control is not focusable and silently
                // blocks submission.
                if (!filterTargetSelect.value) {
                    Kosatka.alert(Kosatka.t('Select a field.'));
                    filterTargetSearch.focus();
                    return;
                }
                var url = filterEditId
                    ? '/admin/config/views/' + VIEW_ID + '/filters/' + filterEditId
                    : '/admin/config/views/' + VIEW_ID + '/filters';
                try {
                    var data = await postForm(url, formToParams(filterForm));
                    filtersList.innerHTML = data.html || '';
                    bootstrap.Modal.getInstance(filterModalEl).hide();
                    refreshPublishStatus();
                } catch (err) {
                    Kosatka.alert(Kosatka.t('Error saving the filter: @error', {'@error': err.message}));
                }
            });
        }

        // ===== Fields card =====
        var fieldModalEl = document.getElementById('fieldModal');
        var fieldForm = document.getElementById('fieldForm');
        var fieldStorageSelect = document.getElementById('field-storage-id');
        var fieldStorageSearch = document.getElementById('field-storage-search');
        setupFieldCombobox(fieldStorageSearch, fieldStorageSelect);
        var fieldStorageSelectWrap = document.getElementById('field-storage-select-wrap');
        var fieldStorageReadonlyWrap = document.getElementById('field-storage-readonly-wrap');
        var fieldStorageReadonly = document.getElementById('field-storage-readonly');
        var fieldEditId = null;
        var fieldImageStyleWrap = document.getElementById('field-image-style-wrap');
        // field-formatter-wrap - link fields only: the formatter and the layout.
        var fieldFormatterWrap = document.getElementById('field-formatter-wrap');
        // field-dates-layout-wrap - date fields only: their several dates' layout.
        var fieldDatesLayoutWrap = document.getElementById('field-dates-layout-wrap');
        var fieldDateFormatWrap = document.getElementById('field-date-format-wrap');
        var fieldTagFormatWrap = document.getElementById('field-tag-format-wrap');
        var fieldLinkToWrap = document.getElementById('field-link-to-wrap');
        // field-trim-length-wrap - text fields only (toggleFieldTypeSections).
        var fieldTrimLengthWrap = document.getElementById('field-trim-length-wrap');
        var fieldLinkToSelect = document.getElementById('field-link-to');
        var fieldLinkToFileOption = fieldLinkToSelect.querySelector('option[data-image-only]');
        var fieldLabelInput = document.getElementById('field-label');

        // Image style and tag format apply only to their own field types: show them
        // only when the selected (add mode) or existing (edit mode) field has that
        // type. The link_to option is hidden for taxonomy (terms link on their own),
        // and its "link to original image" option shows only for image fields, as
        // in updateFilterOperatorOptions.
        function toggleFieldTypeSections(fieldType) {
            fieldImageStyleWrap.classList.toggle('d-none', fieldType !== 'image');
            // Two "layout" selects (link and date): only the shown one is sent.
            if (fieldFormatterWrap) {
                fieldFormatterWrap.classList.toggle('d-none', fieldType !== 'link');
                fieldFormatterWrap.querySelectorAll('select').forEach(function (s) { s.disabled = fieldType !== 'link'; });
            }
            if (fieldDatesLayoutWrap) {
                fieldDatesLayoutWrap.classList.toggle('d-none', fieldType !== 'date');
                fieldDatesLayoutWrap.querySelector('select').disabled = fieldType !== 'date';
            }
            if (fieldDateFormatWrap) {
                var dateFormattable = fieldType === 'date';
                fieldDateFormatWrap.classList.toggle('d-none', !dateFormattable);
                if (!dateFormattable) {
                    document.getElementById('field-date-format').value = '';
                }
            }
            fieldTagFormatWrap.classList.toggle('d-none', fieldType !== 'taxonomy');
            fieldLinkToWrap.classList.toggle('d-none', fieldType === 'taxonomy' || fieldType === 'link');
            fieldLinkToFileOption.hidden = fieldType !== 'image';
            if (fieldType !== 'image' && fieldLinkToSelect.value === 'file') {
                fieldLinkToSelect.value = '';
            }
            // Trimming is for text: hide and reset it for the other field types.
            if (fieldTrimLengthWrap) {
                var trimmable = ['date', 'image', 'file', 'taxonomy', 'bool', 'number', 'list', 'reference', 'link', 'module'].indexOf(fieldType) < 0;
                fieldTrimLengthWrap.classList.toggle('d-none', !trimmable);
                if (!trimmable) {
                    document.getElementById('field-trim-length').value = '';
                }
            }
        }

        // The label placeholder shows the actual render-time default for the chosen
        // field (its name, see views.BuildContentFieldInstances/resolveFieldStorageLabels).
        function updateFieldLabelPlaceholder(defaultLabel) {
            fieldLabelInput.placeholder = defaultLabel ? Kosatka.t('defaults to @label', {'@label': defaultLabel}) : Kosatka.t('optional');
        }

        fieldStorageSelect.addEventListener('change', function () {
            var opt = fieldStorageSelect.selectedOptions[0];
            toggleFieldTypeSections(opt ? opt.getAttribute('data-field-type') : '');
            updateFieldLabelPlaceholder(opt ? opt.getAttribute('data-label') : '');
            showModuleParams(fieldForm, fieldStorageSelect.value, null);
        });

        function openFieldModal(row) {
            if (row) {
                fieldEditId = row.getAttribute('data-field-id');
                document.getElementById('fieldModalTitle').textContent = Kosatka.t('Edit field');
                fieldStorageSelectWrap.classList.add('d-none');
                fieldStorageReadonlyWrap.classList.remove('d-none');
                var fieldStorageLabel = row.getAttribute('data-field-storage-label') || '';
                fieldStorageReadonly.value = fieldStorageLabel;
                document.getElementById('field-label').value = row.getAttribute('data-label') || '';
                document.getElementById('field-show-label').checked = row.getAttribute('data-show-label') === '1';
                document.getElementById('field-image-style').value = row.getAttribute('data-image-style') || '';
                document.getElementById('field-tag-format').value = row.getAttribute('data-tag-format') || '';
                document.getElementById('field-image-display').value = row.getAttribute('data-image-display') || '';
                if (fieldFormatterWrap) {
                    document.getElementById('field-formatter').value = row.getAttribute('data-formatter') || '';
                    document.getElementById('field-layout').value = row.getAttribute('data-layout') || '';
                }
                if (fieldDatesLayoutWrap) {
                    document.getElementById('field-dates-layout').value = row.getAttribute('data-layout') || '';
                }
                document.getElementById('field-css-class').value = row.getAttribute('data-css-class') || '';
                document.getElementById('field-rewrite-html').value = row.getAttribute('data-rewrite-html') || '';
                document.getElementById('field-exclude-from-display').checked = row.getAttribute('data-exclude-from-display') === '1';
                document.getElementById('field-hide-in-overflow').checked = row.getAttribute('data-hide-in-overflow') === '1';
                document.getElementById('field-fallback-key').value = row.getAttribute('data-fallback-field-key') || '';
                var trimLengthInput = document.getElementById('field-trim-length');
                if (trimLengthInput) trimLengthInput.value = (row.getAttribute('data-trim-length') || '0') === '0' ? '' : row.getAttribute('data-trim-length');
                document.getElementById('field-date-format').value = row.getAttribute('data-date-format') || '';
                toggleFieldTypeSections(row.getAttribute('data-field-storage-type') || '');
                fieldLinkToSelect.value = row.getAttribute('data-link-to') || '';
                updateFieldLabelPlaceholder(fieldStorageLabel);
                showModuleParams(fieldForm, row.getAttribute('data-target'), rowParams(row));
            } else {
                fieldEditId = null;
                document.getElementById('fieldModalTitle').textContent = Kosatka.t('Add field');
                fieldForm.reset();
                fieldStorageSelectWrap.classList.remove('d-none');
                fieldStorageReadonlyWrap.classList.add('d-none');
                syncComboboxDisplay(fieldStorageSearch, fieldStorageSelect);
                toggleFieldTypeSections('');
                updateFieldLabelPlaceholder('');
                showModuleParams(fieldForm, '', null);
            }
            new bootstrap.Modal(fieldModalEl).show();
        }

        var fieldAddBtn = document.getElementById('field-add-btn');
        if (fieldAddBtn) fieldAddBtn.addEventListener('click', function () { openFieldModal(null); });

        var fieldsList = document.getElementById('fields-list');
        if (fieldsList) {
            fieldsList.addEventListener('click', function (e) {
                var editBtn = e.target.closest('.field-edit-btn');
                if (editBtn) { openFieldModal(editBtn.closest('tr[data-field-id]')); return; }
                var delBtn = e.target.closest('.field-delete-btn');
                if (delBtn) {
                    var row = delBtn.closest('tr[data-field-id]');
                    var fieldId = row.getAttribute('data-field-id');
                    confirmDelete(Kosatka.t('Delete this field?'), function () {
                        return postDelete('/admin/config/views/' + VIEW_ID + '/fields/' + fieldId + '/delete')
                            .then(function (data) { fieldsList.innerHTML = data.html || ''; refreshPublishStatus(); })
                            .catch(function (err) { Kosatka.alert(Kosatka.t('Error deleting the field: @error', {'@error': err.message})); });
                    });
                }
            });
            setupReorder('fields-list', 'tr[data-field-id]', 'data-field-id',
                '/admin/config/views/' + VIEW_ID + '/fields/reorder',
                function (html) { fieldsList.innerHTML = html; });
        }

        if (fieldForm) {
            fieldForm.addEventListener('submit', async function (e) {
                e.preventDefault();
                // field-storage-id is hidden (see setupFieldCombobox), so native "required"
                // can't apply (a hidden required control silently blocks submission);
                // checked here, in add mode only (edit mode never touches this select).
                if (!fieldEditId && !fieldStorageSelect.value) {
                    Kosatka.alert(Kosatka.t('Select a field.'));
                    fieldStorageSearch.focus();
                    return;
                }
                var url = fieldEditId
                    ? '/admin/config/views/' + VIEW_ID + '/fields/' + fieldEditId
                    : '/admin/config/views/' + VIEW_ID + '/fields';
                try {
                    var data = await postForm(url, formToParams(fieldForm));
                    fieldsList.innerHTML = data.html || '';
                    bootstrap.Modal.getInstance(fieldModalEl).hide();
                    refreshPublishStatus();
                } catch (err) {
                    Kosatka.alert(Kosatka.t('Error saving the field: @error', {'@error': err.message}));
                }
            });
        }

        // ===== Sorts card: the target is immutable on edit, like the Fields card
        // (combobox in add mode, read-only text in edit mode), with Direction
        // instead of Label/ImageStyle. =====
        var sortModalEl = document.getElementById('sortModal');
        var sortForm = document.getElementById('sortForm');
        var sortTargetSelect = document.getElementById('sort-target');
        var sortTargetSearch = document.getElementById('sort-target-search');
        setupFieldCombobox(sortTargetSearch, sortTargetSelect);
        var sortTargetSelectWrap = document.getElementById('sort-target-select-wrap');
        var sortTargetReadonlyWrap = document.getElementById('sort-target-readonly-wrap');
        var sortTargetReadonly = document.getElementById('sort-target-readonly');
        var sortEditId = null;
        sortTargetSelect.addEventListener('change', function () {
            showModuleParams(sortForm, sortTargetSelect.value, null);
        });
        // The offer to visitors: its label and key shown, and required,
        // only when it is ticked.
        var sortExposedCheckbox = document.getElementById('sort-exposed');
        var sortExposedWrap = document.getElementById('sort-exposed-wrap');
        function syncSortExposed() {
            var on = sortExposedCheckbox.checked;
            sortExposedWrap.classList.toggle('d-none', !on);
            sortExposedWrap.querySelectorAll('input').forEach(function (el) { el.required = on; });
        }
        sortExposedCheckbox.addEventListener('change', syncSortExposed);

        function openSortModal(row) {
            if (row) {
                sortEditId = row.getAttribute('data-sort-id');
                document.getElementById('sortModalTitle').textContent = Kosatka.t('Edit sort criterion');
                sortTargetSelectWrap.classList.add('d-none');
                sortTargetReadonlyWrap.classList.remove('d-none');
                sortTargetReadonly.value = row.getAttribute('data-target-label') || '';
                var direction = row.getAttribute('data-direction') || 'desc';
                var directionInput = document.getElementById('sort-direction-' + direction);
                if (directionInput) directionInput.checked = true;
                showModuleParams(sortForm, row.getAttribute('data-target'), rowParams(row));
                var exposedKey = row.getAttribute('data-exposed-key') || '';
                sortExposedCheckbox.checked = exposedKey !== '';
                document.getElementById('sort-exposed-key').value = exposedKey;
                document.getElementById('sort-exposed-label').value = row.getAttribute('data-exposed-label') || '';
            } else {
                sortEditId = null;
                document.getElementById('sortModalTitle').textContent = Kosatka.t('Add sort criterion');
                sortForm.reset();
                sortTargetSelectWrap.classList.remove('d-none');
                sortTargetReadonlyWrap.classList.add('d-none');
                syncComboboxDisplay(sortTargetSearch, sortTargetSelect);
                showModuleParams(sortForm, '', null);
            }
            syncSortExposed();
            new bootstrap.Modal(sortModalEl).show();
        }

        var sortAddBtn = document.getElementById('sort-add-btn');
        if (sortAddBtn) sortAddBtn.addEventListener('click', function () { openSortModal(null); });

        var sortsList = document.getElementById('sorts-list');
        if (sortsList) {
            sortsList.addEventListener('click', function (e) {
                var editBtn = e.target.closest('.sort-edit-btn');
                if (editBtn) { openSortModal(editBtn.closest('tr[data-sort-id]')); return; }
                var delBtn = e.target.closest('.sort-delete-btn');
                if (delBtn) {
                    var row = delBtn.closest('tr[data-sort-id]');
                    var sortId = row.getAttribute('data-sort-id');
                    confirmDelete(Kosatka.t('Delete this sort criterion?'), function () {
                        return postDelete('/admin/config/views/' + VIEW_ID + '/sorts/' + sortId + '/delete')
                            .then(function (data) { sortsList.innerHTML = data.html || ''; refreshPublishStatus(); })
                            .catch(function (err) { Kosatka.alert(Kosatka.t('Error deleting the sort criterion: @error', {'@error': err.message})); });
                    });
                }
            });
            // Order matters here (primary key, then tie-breakers; see
            // ViewSortsReorder), but the drag mechanics match Filters/Fields.
            setupReorder('sorts-list', 'tr[data-sort-id]', 'data-sort-id',
                '/admin/config/views/' + VIEW_ID + '/sorts/reorder',
                function (html) { sortsList.innerHTML = html; });
        }

        if (sortForm) {
            sortForm.addEventListener('submit', async function (e) {
                e.preventDefault();
                if (!sortEditId && !sortTargetSelect.value) {
                    Kosatka.alert(Kosatka.t('Select a field.'));
                    sortTargetSearch.focus();
                    return;
                }
                var url = sortEditId
                    ? '/admin/config/views/' + VIEW_ID + '/sorts/' + sortEditId
                    : '/admin/config/views/' + VIEW_ID + '/sorts';
                try {
                    var data = await postForm(url, formToParams(sortForm));
                    sortsList.innerHTML = data.html || '';
                    bootstrap.Modal.getInstance(sortModalEl).hide();
                    refreshPublishStatus();
                } catch (err) {
                    Kosatka.alert(Kosatka.t('Error saving the sort criterion: @error', {'@error': err.message}));
                }
            });
        }

        // ===== Live preview: reads the settings modal's current (possibly unsaved)
        // values from the DOM. The preview's pager and glossary bar are AJAX-only
        // (a preview has no URL): they re-POST the same draft settings with another
        // page or letter instead of navigating.
        var previewBtn = document.getElementById('preview-btn');
        // The list renders in a frame (admin/views/page.html) with the site's
        // stylesheets and the active theme (data.head, view_preview_head);
        // its body wraps the list in the page's real id "view-list-container",
        // so Custom CSS scoped to "#view-list-container ..." applies too.
        var previewFrame = document.getElementById('view-preview-frame');
        var previewStatus = document.getElementById('view-preview-status');
        var previewLetter = '';
        // previewExposedFilters - the preview's exposed-filters values, re-POSTed
        // instead of navigating with a "?param=value" query. Replaced wholesale on
        // every submit/reset, matching the public page's GET semantics (an unset
        // field is absent).
        var previewExposedFilters = {};

        async function loadPreview(page, letter) {
            if (letter !== undefined) previewLetter = letter;
            previewStatus.className = 'text-muted';
            previewStatus.textContent = Kosatka.t('Loading…');
            previewStatus.hidden = false;
            var displayStyleInput = settingsForm.querySelector('input[name="display_style"]:checked');
            // Sorts (like filters and fields) are saved immediately by their own modal,
            // so the preview endpoint reads them from the DB.
            // header_text/footer_text: TinyMCE replaces the <textarea> with an iframe,
            // so read the live content with tinymce.get(id).getContent(), falling back
            // to the textarea when TinyMCE hasn't loaded yet.
            function currentRichText(id) {
                var editor = window.tinymce && window.tinymce.get(id);
                if (editor) return editor.getContent();
                var el = document.getElementById(id);
                return el ? el.value : '';
            }
            // settings-grid-columns is hidden for comment-based views (settings_modal.html):
            // comment_list.html always renders a plain list.
            var gridColumnsEl = document.getElementById('settings-grid-columns');
            var overflowAfterEl = document.getElementById('settings-overflow-after');
            var overflowHeaderTextEl = document.getElementById('settings-overflow-header-text');
            var carouselSlidesEl = document.getElementById('settings-carousel-slides-per-view');
            var carouselAutoplayEl = document.getElementById('settings-carousel-autoplay');
            var carouselLoopEl = document.getElementById('settings-carousel-loop');
            var carouselNavEl = document.getElementById('settings-carousel-show-navigation');
            var carouselPagingEl = document.getElementById('settings-carousel-show-pagination');
            var carouselLinkFieldEl = document.getElementById('settings-carousel-link-field');
            var carouselEffectInput = settingsForm.querySelector('input[name="carousel_effect"]:checked');
            var payload = {
                display_style: displayStyleInput ? displayStyleInput.value : 'grid',
                grid_columns: gridColumnsEl ? (parseInt(gridColumnsEl.value, 10) || 0) : 0,
                overflow_after: overflowAfterEl ? (parseInt(overflowAfterEl.value, 10) || 0) : 0,
                overflow_header_text: overflowHeaderTextEl ? overflowHeaderTextEl.value : '',
                carousel_slides_per_view: carouselSlidesEl ? (parseInt(carouselSlidesEl.value, 10) || 0) : 0,
                carousel_autoplay: carouselAutoplayEl ? carouselAutoplayEl.checked : false,
                carousel_loop: carouselLoopEl ? carouselLoopEl.checked : false,
                carousel_show_navigation: carouselNavEl ? carouselNavEl.checked : false,
                carousel_show_pagination: carouselPagingEl ? carouselPagingEl.checked : false,
                carousel_link_field_storage_id: carouselLinkFieldEl ? (parseInt(carouselLinkFieldEl.value, 10) || 0) : 0,
                carousel_effect: carouselEffectInput ? carouselEffectInput.value : 'slide',
                show_pagination: document.getElementById('settings-show-pagination').checked,
                page_size: parseInt(document.getElementById('settings-page-size').value, 10) || 0,
                header_text: currentRichText('settings-header-text'),
                footer_text: currentRichText('settings-footer-text'),
                custom_css: document.getElementById('settings-custom-css') ? document.getElementById('settings-custom-css').value : '',
                row_template: document.getElementById('settings-row-template') ? document.getElementById('settings-row-template').value : '',
                group_by_field: document.getElementById('settings-group-by-field') ? document.getElementById('settings-group-by-field').value : '',
                group_date_format: document.getElementById('settings-group-date-format') ? document.getElementById('settings-group-date-format').value : '',
                page: page || 1,
                letter: previewLetter,
                exposed_filters: previewExposedFilters
            };
            try {
                var data = await postJSON('/admin/config/views/' + VIEW_ID + '/preview', payload);
                // app.js in the frame's head starts carousels on DOMContentLoaded;
                // links open in a new tab, not inside the frame.
                previewFrame.srcdoc = '<!doctype html><html lang="' + (document.documentElement.lang || '') + '"><head>' +
                    '<meta charset="utf-8"><base target="_blank">' + (data.head || '') + '</head>' +
                    '<body><div id="view-list-container">' + (data.html || '') + '</div></body></html>';
                previewStatus.hidden = true;
            } catch (err) {
                previewStatus.className = 'alert alert-danger';
                previewStatus.textContent = Kosatka.t('Preview error: @error', {'@error': err.message});
                previewStatus.hidden = false;
            }
        }

        // fitPreviewFrame keeps the frame as tall as its content.
        var previewResize = null;
        function fitPreviewFrame() {
            var doc = previewFrame.contentDocument;
            if (!doc || !doc.body) return;
            previewFrame.style.height = Math.max(300, doc.documentElement.scrollHeight) + 'px';
        }

        // The preview under the editor: drawn when the page opens, redrawn by
        // "Refresh" and - while "Refresh automatically" is on (remembered in
        // this browser) - after every change of the display and as the
        // settings modal's values change.
        var previewAuto = document.getElementById('view-preview-auto');
        var autoKey = 'kosatka.viewPreviewAuto';
        if (previewAuto) {
            try {
                if (localStorage.getItem(autoKey) === '0') previewAuto.checked = false;
            } catch (e) { /* storage blocked: the default stays */ }
            previewAuto.addEventListener('change', function () {
                try {
                    localStorage.setItem(autoKey, previewAuto.checked ? '1' : '0');
                } catch (e) { /* storage blocked */ }
                if (previewAuto.checked) loadPreview(1);
            });
        }
        function autoPreviewOn() {
            return !previewAuto || previewAuto.checked;
        }
        if (previewBtn) {
            previewBtn.addEventListener('click', function () {
                loadPreview(1, '');
            });
        }
        if (previewFrame) {
            loadPreview(1, '');
            document.addEventListener('kosatka:view-changed', function () {
                if (autoPreviewOn()) loadPreview(1);
            });
            var settingsTimer = null;
            if (settingsForm) {
                settingsForm.addEventListener('input', function () {
                    if (!autoPreviewOn()) return;
                    clearTimeout(settingsTimer);
                    settingsTimer = setTimeout(function () { loadPreview(1); }, 600);
                });
            }
        }
        // bindPreviewEvents - the preview's exposed filters, glossary and pager
        // work inside the frame: they re-POST the draft settings instead of
        // navigating (a preview has no URL).
        function bindPreviewEvents(previewBody) {
            // Exposed-filters form submit: collect every named field (same shape as the
            // public page's query string) and reload the preview from page 1.
            previewBody.addEventListener('submit', function (e) {
                var form = e.target.closest('.preview-exposed-filters-form');
                if (!form) return;
                e.preventDefault();
                previewExposedFilters = {};
                Array.prototype.forEach.call(form.elements, function (el) {
                    if (el.name && el.value) previewExposedFilters[el.name] = el.value;
                });
                loadPreview(1);
            });
            previewBody.addEventListener('click', function (e) {
                if (e.target.closest('.preview-exposed-filters-reset')) {
                    previewExposedFilters = {};
                    loadPreview(1);
                    return;
                }
                var letterLink = e.target.closest('.preview-glossary-letter');
                if (letterLink) {
                    e.preventDefault();
                    loadPreview(1, letterLink.getAttribute('data-target-letter') || '');
                    return;
                }
                var link = e.target.closest('.pagination a[data-page]');
                if (!link) return;
                e.preventDefault();
                var page = parseInt(link.getAttribute('data-page'), 10) || 1;
                loadPreview(page);
            });
        }

        if (previewFrame) {
            previewFrame.addEventListener('load', function () {
                var doc = previewFrame.contentDocument;
                if (!doc || !doc.body) return;
                bindPreviewEvents(doc);
                fitPreviewFrame();
                if (previewResize) previewResize.disconnect();
                if (window.ResizeObserver) {
                    previewResize = new ResizeObserver(fitPreviewFrame);
                    previewResize.observe(doc.body);
                }
            });
        }
    });
})();
