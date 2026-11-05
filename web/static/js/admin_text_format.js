// admin_text_format.js - a text format's page (admin/text-formats/edit.html):
// the buttons checked ([data-toolbar-group] - the toolbar's groups) make
// the format's toolbar and plugins (its hidden inputs), and the preview
// editor shows them as they change.
document.addEventListener('DOMContentLoaded', function() {
    var toolbarInput = document.getElementById('tinymce_toolbar');
    var pluginsInput = document.getElementById('tinymce_plugins');
    var options = document.querySelectorAll('.tb-option');

    function rebuild() {
        var toolbarParts = ['undo redo'];
        var pluginsSet = {};
        document.querySelectorAll('[data-toolbar-group]').forEach(function(group) {
            var buttons = [];
            group.querySelectorAll('.tb-option:checked').forEach(function(cb) {
                buttons.push(cb.dataset.button);
                if (cb.dataset.plugin) {
                    pluginsSet[cb.dataset.plugin] = true;
                }
            });
            if (buttons.length) {
                toolbarParts.push(buttons.join(' '));
            }
        });
        var toolbar = toolbarParts.join(' | ');
        var plugins = Object.keys(pluginsSet).join(' ');
        toolbarInput.value = toolbar;
        pluginsInput.value = plugins;
        reinitPreview(plugins, toolbar);
    }

    function reinitPreview(plugins, toolbar) {
        var existing = tinymce.get('tinymce-format-preview');
        var content = existing ? existing.getContent() : document.getElementById('tinymce-format-preview').value;
        if (existing) {
            existing.remove();
        }
        tinymce.init(Kosatka.editorConfig({
            selector: '#tinymce-format-preview',
            plugins: plugins,
            toolbar: toolbar,
            height: 300,
            branding: false,
            promotion: false
        })).then(function(editors) {
            if (editors && editors[0]) {
                editors[0].setContent(content);
            }
        });
    }

    options.forEach(function(cb) {
        cb.addEventListener('change', rebuild);
    });

    // Initial preview from the saved config, not the checkbox state -
    // avoids flashing the default toolbar on load.
    reinitPreview(pluginsInput.value, toolbarInput.value || 'undo redo');
});
