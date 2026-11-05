// ===== Editor =====
// Kosatka.loadEditor(textarea) - TinyMCE on a textarea whose
// data-editor-config holds its settings (JSON: preset, plugins, toolbar,
// height…), through Kosatka.editorConfig and
// cmsTinyMCEUserFiles.mergeConfig (uploads, the file picker, links as
// typed, the theme's stylesheets): the page includes tinymce-user-files
// (partials/tinymce_user_files.html); TinyMCE itself loads on first use.
// Resolves with the editor; a second call for the same textarea gets the
// same one. editors - every textarea[data-editor-autoload] gets its
// editor right away.
var tinymceLoading = null;
var editorsLoading = new WeakMap();

// Kosatka.loadTinyMCE - resolves once TinyMCE is on the page.
Kosatka.loadTinyMCE = function () {
    if (window.tinymce) {
        return Promise.resolve();
    }
    if (!tinymceLoading) {
        tinymceLoading = new Promise(function (resolve, reject) {
            var script = document.createElement('script');
            script.src = '/static/js/tinymce/tinymce.min.js';
            script.onload = function () { resolve(); };
            script.onerror = function () { tinymceLoading = null; reject(new Error('TinyMCE failed to load')); };
            document.head.appendChild(script);
        });
    }
    return tinymceLoading;
};

// Kosatka.editorPresets - settings several forms share, by name: a
// config's "preset" starts from one, its own keys win. comment - comments,
// replies, quotes; text - longer texts (headings, alignment, images).
Kosatka.editorPresets = {
    comment: {
        plugins: 'link image code lists',
        toolbar: 'undo redo | bold italic | bullist numlist | link | code',
        height: 300
    },
    text: {
        plugins: 'link image code lists',
        // "blocks", not the pre-6 name "formatselect": TinyMCE silently
        // drops unknown toolbar items.
        toolbar: 'undo redo | blocks | bold italic | alignleft aligncenter alignright | bullist numlist | link image | code',
        height: 400
    }
};

// Kosatka.editorConfig(cfg) - cfg over its preset's settings, with what
// every editor has: no menu bar, the insertbreak button (inserts
// <!--break-->, the teaser's end) for a toolbar that lists it.
Kosatka.editorConfig = function (cfg) {
    var out = Object.assign({ license_key: 'gpl', menubar: false }, Kosatka.editorPresets[cfg.preset] || {}, cfg);
    delete out.preset;
    var setup = out.setup;
    out.setup = function (editor) {
        editor.ui.registry.addButton('insertbreak', {
            text: '<!--break-->',
            tooltip: Kosatka.t('Insert a break (<!--break-->)'),
            onAction: function () { editor.insertContent('<!--break-->'); }
        });
        if (setup) {
            setup(editor);
        }
    };
    return out;
};

// withUserFiles - cfg with the uploads and the file picker when the page
// has them (tinymce-user-files).
function withUserFiles(cfg, formatId) {
    return window.cmsTinyMCEUserFiles ? window.cmsTinyMCEUserFiles.mergeConfig(cfg, formatId) : cfg;
}

Kosatka.loadEditor = function (textarea) {
    if (editorsLoading.has(textarea)) {
        return editorsLoading.get(textarea);
    }
    var loading = Kosatka.loadTinyMCE().then(function () {
        var config = {};
        try {
            config = JSON.parse(textarea.getAttribute('data-editor-config') || '{}');
        } catch (err) {
            console.error('data-editor-config:', err);
        }
        return new Promise(function (resolve) {
            config.target = textarea;
            config.setup = function (editor) {
                editor.on('init', function () { resolve(editor); });
            };
            tinymce.init(withUserFiles(Kosatka.editorConfig(config)));
        });
    });
    editorsLoading.set(textarea, loading);
    loading.catch(function () { editorsLoading.delete(textarea); });
    return loading;
};

Kosatka.behaviors.editors = {
    attach: function (context) {
        Kosatka.once('editor', 'textarea[data-editor-autoload]', context).forEach(function (textarea) {
            Kosatka.loadEditor(textarea);
        });
    }
};

// ===== Text format picker =====
// A select.text-format-picker (fields/input.html, textFormatPicker) puts
// the editor on its textarea (data-target-field) while the selected
// format has one (its option's data-rich-editor="1"), with the format's
// data-plugins, data-toolbar and data-external-plugins (the modules' text
// filters' buttons), and takes it off for a format without; the text is
// kept across. The editor shows the text as the theme does (a
// .node-content body). The select's "text-format-detach" event takes the
// editor off, its "change" puts it back (a block moved).
function textFormatAttachEditor(targetId, plugins, toolbar, formatId, external) {
    var existing = tinymce.get(targetId);
    var textarea = document.getElementById(targetId);
    var content = existing ? existing.getContent() : (textarea ? textarea.value : '');
    if (existing) {
        existing.remove();
    }
    var cfg = { selector: '#' + targetId, height: 400, body_class: 'node-content' };
    // A format without its own plugins or toolbar keeps TinyMCE's
    // (an empty toolbar would hide it).
    if (plugins) {
        cfg.plugins = plugins;
    }
    if (toolbar) {
        cfg.toolbar = toolbar;
    }
    if (external) {
        try {
            cfg.external_plugins = JSON.parse(external);
        } catch (e) {
            // A broken list leaves the editor without those buttons.
        }
    }
    tinymce.init(withUserFiles(Kosatka.editorConfig(cfg), formatId)).then(function (editors) {
        if (editors && editors[0]) {
            editors[0].setContent(content);
        }
    });
}

function textFormatDetachEditor(targetId) {
    var existing = window.tinymce && tinymce.get(targetId);
    if (!existing) {
        return;
    }
    var textarea = document.getElementById(targetId);
    if (textarea) {
        textarea.value = existing.getContent();
    }
    existing.remove();
}

Kosatka.behaviors.textFormatPicker = {
    attach: function (context) {
        Kosatka.once('text-format-picker', '.text-format-picker', context).forEach(function (picker) {
            var targetId = picker.dataset.targetField;

            function applySelectedFormat() {
                var opt = picker.options[picker.selectedIndex];
                if (!opt) {
                    return;
                }
                if (opt.dataset.richEditor !== '1') {
                    textFormatDetachEditor(targetId);
                    return;
                }
                Kosatka.loadTinyMCE().then(function () {
                    // Another format may have been picked while TinyMCE loaded.
                    if (picker.options[picker.selectedIndex] === opt) {
                        textFormatAttachEditor(targetId, opt.dataset.plugins || '', opt.dataset.toolbar || '', picker.value, opt.dataset.externalPlugins || '');
                    }
                });
            }

            // Initial state, same as on change: the default-selected option
            // (the value's saved format, or the field's lowest-weight allowed one).
            applySelectedFormat();
            picker.addEventListener('change', applySelectedFormat);
            picker.addEventListener('text-format-detach', function () { textFormatDetachEditor(targetId); });
        });
    }
};
