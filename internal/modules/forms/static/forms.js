// Forms module (entity.ModuleForms), loaded after app.js on the pages
// that attach it ({{attach}}: its forms, the "Form" block).

// ===== Conditions =====
// An element with data-visible-if="element_<id>" is shown only when that
// element has data-visible-value (empty - any answer); a hidden element's
// inputs are disabled, so they aren't sent or required, and an element
// depending on it hides too. The server checks the same.
function webformInitConditions(root) {
    root.querySelectorAll('form.webform-form').forEach(function (form) {
        var items = form.querySelectorAll('[data-visible-if]');
        if (!items.length || form.hasAttribute('data-conditions')) {
            return;
        }
        form.setAttribute('data-conditions', '1');
        function values(name) {
            var out = [];
            form.querySelectorAll('[name="' + name + '"]').forEach(function (el) {
                if (el.disabled || ((el.type === 'checkbox' || el.type === 'radio') && !el.checked)) {
                    return;
                }
                if (el.value.trim() !== '') {
                    out.push(el.value);
                }
            });
            return out;
        }
        function update() {
            items.forEach(function (item) {
                var vals = values(item.getAttribute('data-visible-if'));
                var want = item.getAttribute('data-visible-value');
                var show = want === '' ? vals.length > 0 : vals.indexOf(want) >= 0;
                item.hidden = !show;
                item.querySelectorAll('input, select, textarea').forEach(function (el) {
                    el.disabled = !show;
                });
            });
        }
        form.addEventListener('change', update);
        form.addEventListener('input', update);
        update();
    });
}

Kosatka.behaviors.webformConditions = {
    attach: function (context) {
        webformInitConditions(context);
    }
};

// ===== Prefill and source =====
// An element with data-prefill="<parameter>" takes the address's value of
// that query parameter when it has no answer yet (a form shown again with
// errors keeps what was typed): a text - as it is, a list or radios - the
// option with that value, checkboxes - the options named (repeated or
// comma-separated). The server checks the answer like any other.
// webform_source gets the page the form is sent from: the page it's
// embedded in, or - on the form's own page - the site's page that linked
// to it.
function webformInitPrefill(root) {
    var params = new URLSearchParams(window.location.search);
    root.querySelectorAll('form.webform-form').forEach(function (form) {
        if (form.hasAttribute('data-prefilled')) {
            return;
        }
        form.setAttribute('data-prefilled', '1');
        var source = form.querySelector('[data-webform-source]');
        if (source && source.value === '') {
            var own = source.getAttribute('data-form-path');
            if (window.location.pathname !== own) {
                source.value = window.location.pathname + window.location.search;
            } else if (document.referrer) {
                try {
                    var ref = new URL(document.referrer);
                    if (ref.origin === window.location.origin && ref.pathname !== own) {
                        source.value = ref.pathname + ref.search;
                    }
                } catch (e) {
                    // Not an address: no source.
                }
            }
        }
        var filled = false;
        form.querySelectorAll('[data-prefill]').forEach(function (item) {
            var values = params.getAll(item.getAttribute('data-prefill'));
            if (!values.length) {
                return;
            }
            var inputs = Array.prototype.slice.call(item.querySelectorAll('input, select, textarea'));
            var answered = inputs.some(function (el) {
                return (el.type === 'checkbox' || el.type === 'radio') ? el.checked : el.value !== '';
            });
            if (answered) {
                return;
            }
            var wanted = [];
            values.forEach(function (v) {
                v.split(',').forEach(function (part) {
                    part = part.trim();
                    if (part !== '') {
                        wanted.push(part);
                    }
                });
            });
            inputs.forEach(function (el) {
                if (el.type === 'checkbox') {
                    el.checked = wanted.indexOf(el.value) >= 0;
                } else if (el.type === 'radio') {
                    el.checked = el.value === values[0];
                } else if (el.tagName === 'SELECT') {
                    if (Array.prototype.some.call(el.options, function (o) { return o.value === values[0]; })) {
                        el.value = values[0];
                    }
                } else {
                    el.value = values[0];
                }
            });
            filled = true;
        });
        if (filled) {
            // The conditions see the answers.
            form.dispatchEvent(new Event('change'));
        }
    });
}

Kosatka.behaviors.webformPrefill = {
    attach: function (context) {
        webformInitPrefill(context);
    }
};

// ===== File size =====
// A file past the site's upload limit (data-max-size) is dropped before
// sending, with the server's message: the visitor doesn't upload it only
// to see it refused.
document.addEventListener('change', function (e) {
    var input = e.target;
    if (!input.matches || !input.matches('form.webform-form input[type="file"][data-max-size]')) {
        return;
    }
    var max = parseInt(input.getAttribute('data-max-size'), 10);
    var tooLarge = Array.prototype.some.call(input.files || [], function (f) {
        return max > 0 && f.size > max;
    });
    var note = input.parentNode.querySelector('.webform-file-error');
    if (tooLarge) {
        input.value = '';
        if (!note) {
            note = document.createElement('div');
            note.className = 'invalid-feedback d-block webform-file-error';
            input.insertAdjacentElement('afterend', note);
        }
        note.textContent = Kosatka.t('The file is too large.');
    } else if (note) {
        note.remove();
    }
});

// ===== The "Form" block =====
// A block placed with a form (blocks/webform.html) is an empty
// [data-webform-embed] element: its form is loaded from the module
// (/node/<id>/form/embed), so a cached page still shows each visitor what
// they may send. Submitting goes to the form's own page.
Kosatka.behaviors.webformBlocks = {
    attach: function (context) {
        Kosatka.once('webform-embed', '[data-webform-embed]', context).forEach(loadWebformBlock);
    }
};

function loadWebformBlock(block) {
    // Scripts inserted as HTML don't run: the reCAPTCHA script is added
    // anew (it renders the widgets present when it loads) or, once loaded,
    // asked to render the new widget.
    function startCaptcha(root) {
        var widget = root.querySelector('.g-recaptcha');
        if (!widget) {
            return;
        }
        if (window.grecaptcha && window.grecaptcha.render) {
            window.grecaptcha.render(widget);
            return;
        }
        var old = root.querySelector('script[src*="recaptcha"]');
        if (old && !document.querySelector('script[data-webform-captcha]')) {
            var s = document.createElement('script');
            s.src = old.src;
            s.async = true;
            s.defer = true;
            s.setAttribute('data-webform-captcha', '1');
            document.head.appendChild(s);
        }
    }
    fetch(block.getAttribute('data-webform-embed'), { credentials: 'same-origin' })
        .then(function (resp) {
            if (!resp.ok) {
                throw new Error('HTTP ' + resp.status);
            }
            return resp.text();
        })
        .then(function (html) {
            block.innerHTML = html;
            startCaptcha(block);
            Kosatka.attachBehaviors(block);
        })
        .catch(function () {
            block.remove();
        });
}

// ===== Submissions =====
// webformSelectAll - the submissions' list (webform/submissions.html): its
// .webform-select-all checks or unchecks every submission for deleting.
Kosatka.behaviors.webformSelectAll = {
    attach: function (context) {
        Kosatka.once('webform-select-all', '.webform-select-all', context).forEach(function (all) {
            all.addEventListener('change', function () {
                document.querySelectorAll('.webform-submissions-delete input[name="ids"]').forEach(function (cb) { cb.checked = all.checked; });
            });
        });
    }
};
