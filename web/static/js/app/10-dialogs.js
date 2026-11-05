// ===== Dialogs =====
// Kosatka.confirm(message), Kosatka.alert(message), Kosatka.prompt(message,
// value) - the site's own dialog (a <dialog>, .kosatka-dialog: the theme
// styles it), not the browser's confirm/alert/prompt, which block the
// page, can't be styled or have their buttons translated, and which a
// browser may stop showing. Each returns a Promise: confirm - true for
// OK; prompt - the text, or null when cancelled; alert - once closed.
// Escape or a click outside the box cancels. confirm's okText names its
// OK button ("Yes, delete"), OK without one.
Kosatka.dialog = function (opts) {
    // Kosatka.t written out: the JS dictionary collects its literals.
    var okText = opts.okText || (Kosatka.t ? Kosatka.t('OK') : 'OK');
    var cancelText = Kosatka.t ? Kosatka.t('Cancel') : 'Cancel';
    return new Promise(function (resolve) {
        var dialog = document.createElement('dialog');
        dialog.className = 'kosatka-dialog';
        var form = document.createElement('form');
        form.method = 'dialog';
        var message = document.createElement('p');
        message.className = 'kosatka-dialog-message';
        message.textContent = opts.message;
        form.appendChild(message);
        var input = null;
        if (opts.input) {
            input = document.createElement('input');
            input.type = 'text';
            input.className = 'form-control kosatka-dialog-input';
            input.value = opts.value || '';
            input.setAttribute('aria-label', opts.message);
            form.appendChild(input);
        }
        var buttons = document.createElement('div');
        buttons.className = 'kosatka-dialog-buttons';
        // OK is the form's submit (Enter in the field), Cancel a plain
        // button.
        var ok = document.createElement('button');
        ok.type = 'submit';
        ok.value = 'ok';
        ok.className = 'btn btn-primary';
        ok.textContent = okText;
        if (opts.cancel) {
            var cancel = document.createElement('button');
            cancel.type = 'button';
            cancel.className = 'btn btn-secondary';
            cancel.textContent = cancelText;
            cancel.addEventListener('click', function () { dialog.close('cancel'); });
            buttons.appendChild(cancel);
        }
        buttons.appendChild(ok);
        form.appendChild(buttons);
        dialog.appendChild(form);
        dialog.addEventListener('click', function (e) {
            if (e.target === dialog) {
                dialog.close('cancel');
            }
        });
        dialog.addEventListener('close', function () {
            var confirmed = dialog.returnValue === 'ok';
            dialog.remove();
            resolve(opts.input ? (confirmed ? input.value : null) : confirmed);
        });
        document.body.appendChild(dialog);
        dialog.showModal();
        (input || ok).focus();
        if (input) {
            input.select();
        }
    });
};
Kosatka.confirm = function (message, okText) { return Kosatka.dialog({ message: message, cancel: true, okText: okText }); };
Kosatka.alert = function (message) { return Kosatka.dialog({ message: message }); };
Kosatka.prompt = function (message, value) { return Kosatka.dialog({ message: message, value: value, input: true, cancel: true }); };

// ===== Confirm =====
// data-confirm="<question>" - the action waits for the visitor's yes
// (Kosatka.confirm): a form's submit, a button's or a link's click. Caught
// on the document, so it works in markup added later and in the admin.
// On yes the action is repeated, marked confirmed so it isn't asked again.
document.addEventListener('submit', function (e) {
    var form = e.target;
    if (!form.hasAttribute || !form.hasAttribute('data-confirm')) {
        return;
    }
    if (form.dataset.confirmed === '1') {
        delete form.dataset.confirmed;
        return;
    }
    e.preventDefault();
    var submitter = e.submitter && e.submitter.form === form ? e.submitter : null;
    Kosatka.confirm(form.getAttribute('data-confirm')).then(function (yes) {
        if (!yes) {
            return;
        }
        form.dataset.confirmed = '1';
        if (form.requestSubmit) {
            form.requestSubmit(submitter || undefined);
        } else {
            form.submit();
        }
    });
});
document.addEventListener('click', function (e) {
    var el = e.target.closest('[data-confirm]');
    if (!el || el.tagName === 'FORM') {
        return;
    }
    if (el.dataset.confirmed === '1') {
        delete el.dataset.confirmed;
        return;
    }
    e.preventDefault();
    Kosatka.confirm(el.getAttribute('data-confirm')).then(function (yes) {
        if (yes) {
            el.dataset.confirmed = '1';
            el.click();
        }
    });
});


// ===== Windows =====
// Kosatka.window(opts) - a window over the page for a tool bigger than a
// dialog (the editor's "Insert into the text", the media library's
// picker): a head with opts.title and ×, the body, the foot for the
// buttons (button(text, primary) adds one); opts.className gives it its
// own look (overlays.css). Escape, a mouse press beside it, × and close()
// shut it; then opts.onClose, or the focus goes back to opts.opener (the
// element focused when it opened by default). opts.onKey gets the other
// keys pressed while it is open.
var windowCount = 0;

Kosatka.window = function (opts) {
    var opener = opts.opener || document.activeElement;
    var overlay = document.createElement('div');
    overlay.className = 'kosatka-window-overlay';
    var dialog = document.createElement('div');
    dialog.className = 'kosatka-window' + (opts.className ? ' ' + opts.className : '');
    dialog.setAttribute('role', 'dialog');
    dialog.setAttribute('aria-modal', 'true');
    var head = document.createElement('div');
    head.className = 'kosatka-window-head';
    var title = document.createElement('h2');
    title.id = 'kosatka-window-title-' + (++windowCount);
    title.textContent = opts.title;
    dialog.setAttribute('aria-labelledby', title.id);
    var closeBtn = document.createElement('button');
    closeBtn.type = 'button';
    closeBtn.className = 'kosatka-window-close';
    closeBtn.textContent = '×';
    closeBtn.setAttribute('aria-label', Kosatka.t('Close'));
    head.append(title, closeBtn);
    var body = document.createElement('div');
    body.className = 'kosatka-window-body';
    var foot = document.createElement('div');
    foot.className = 'kosatka-window-foot';
    dialog.append(head, body, foot);
    overlay.appendChild(dialog);
    document.body.appendChild(overlay);

    function onKey(e) {
        if (e.key === 'Escape') {
            e.preventDefault();
            close();
        } else if (opts.onKey) {
            opts.onKey(e);
        }
    }
    function close() {
        overlay.remove();
        document.removeEventListener('keydown', onKey, true);
        if (opts.onClose) {
            opts.onClose();
        } else if (opener && opener.focus) {
            opener.focus();
        }
    }
    closeBtn.addEventListener('click', close);
    overlay.addEventListener('mousedown', function (e) {
        if (e.target === overlay) {
            close();
        }
    });
    document.addEventListener('keydown', onKey, true);

    return {
        dialog: dialog,
        body: body,
        foot: foot,
        close: close,
        button: function (text, primary) {
            var b = document.createElement('button');
            b.type = 'button';
            b.className = 'kosatka-window-btn' + (primary ? ' is-primary' : '');
            b.textContent = text;
            foot.appendChild(b);
            return b;
        }
    };
};

// ===== Toasts =====
// Kosatka.toast(message, type, ms) - a short note in the bottom corner of
// the page that goes by itself after ms (3 s by default): type info (the
// default), success, warning or error gives its colour (overlays.css);
// screen readers read it.
Kosatka.toast = function (message, type, ms) {
    var stack = document.querySelector('.kosatka-toasts');
    if (!stack) {
        stack = document.createElement('div');
        stack.className = 'kosatka-toasts';
        document.body.appendChild(stack);
    }
    var toast = document.createElement('div');
    toast.className = 'kosatka-toast kosatka-toast--' + (type || 'info');
    toast.setAttribute('role', type === 'error' ? 'alert' : 'status');
    toast.textContent = message;
    stack.appendChild(toast);
    // Shown on the next frame, so that it slides in.
    requestAnimationFrame(function () {
        requestAnimationFrame(function () { toast.classList.add('is-shown'); });
    });
    setTimeout(function () {
        toast.classList.remove('is-shown');
        setTimeout(function () { toast.remove(); }, 300);
    }, ms || 3000);
};
