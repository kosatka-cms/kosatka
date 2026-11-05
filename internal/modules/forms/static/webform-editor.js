// webform-editor.js - the form's questions in its content form
// (node/edit_webform.html): rows added from #webform-element-template,
// removed and moved up or down; a row's settings follow its type; "Show
// only if" offers the earlier rows by their current labels. Loaded after
// app.js (defer): Kosatka.t.
document.addEventListener('DOMContentLoaded', function () {
  var list = document.getElementById('webform-elements');
  var add = document.getElementById('webform-add-element');
  var tpl = document.getElementById('webform-element-template');
  if (!list || !add || !tpl) return;
  var next = 1;
  function syncOptions(row) {
    var type = row.querySelector('.webform-element-type').value;
    row.querySelector('.webform-element-options').hidden = ['select', 'radios', 'checkboxes'].indexOf(type) < 0;
    // A consent is always required: its link instead of the checkbox.
    row.querySelector('.webform-element-link').hidden = type !== 'consent';
    row.querySelector('.webform-element-ext').hidden = type !== 'file';
    row.querySelector('.webform-element-range').hidden = type !== 'number';
    row.querySelector('.webform-element-required').hidden = type === 'consent';
    // A file or a consent can't be answered from the address.
    row.querySelector('.webform-element-prefill').hidden = type === 'file' || type === 'consent';
  }
  function addRow() {
    var key = 'new' + (next++);
    var wrap = document.createElement('div');
    wrap.innerHTML = tpl.innerHTML.split('__KEY__').join(key);
    var row = wrap.firstElementChild;
    list.appendChild(row);
    syncOptions(row);
    row.querySelector('input[type=text]').focus();
  }
  // "Show only if": the earlier rows, by their current labels; a row
  // keeps its choice while it stays earlier.
  function refreshTargets() {
    var rows = list.querySelectorAll('.webform-element-row');
    rows.forEach(function (row, i) {
      var sel = row.querySelector('.webform-element-if-target');
      var current = sel.value || sel.getAttribute('data-current') || '';
      sel.removeAttribute('data-current');
      while (sel.options.length > 1) sel.remove(1);
      for (var j = 0; j < i; j++) {
        var key = rows[j].querySelector('input[name="element_row"]').value;
        var label = rows[j].querySelector('input[name^="element_label_"]').value || (Kosatka.t('Question') + ' ' + (j + 1));
        sel.add(new Option(label, key, false, key === current));
      }
      row.querySelector('.webform-element-if').hidden = i === 0;
    });
  }
  add.addEventListener('click', function () { addRow(); refreshTargets(); });
  list.addEventListener('change', function (e) {
    if (e.target.classList.contains('webform-element-type')) syncOptions(e.target.closest('.webform-element-row'));
  });
  list.addEventListener('click', function (e) {
    var row = e.target.closest('.webform-element-row');
    if (!row) return;
    if (e.target.classList.contains('webform-element-remove')) {
      row.remove();
    } else if (e.target.classList.contains('webform-element-up') && row.previousElementSibling) {
      list.insertBefore(row, row.previousElementSibling);
    } else if (e.target.classList.contains('webform-element-down') && row.nextElementSibling) {
      list.insertBefore(row.nextElementSibling, row);
    } else {
      return;
    }
    refreshTargets();
  });
  list.addEventListener('input', function (e) {
    if (e.target.name && e.target.name.indexOf('element_label_') === 0) refreshTargets();
  });
  list.querySelectorAll('.webform-element-row').forEach(syncOptions);
  if (!list.querySelector('.webform-element-row')) addRow();
  refreshTargets();
});
