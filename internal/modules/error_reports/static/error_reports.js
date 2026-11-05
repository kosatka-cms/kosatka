// Error reports module (the error_reports module), loaded after app.js
// (attached by its templates, {{attach}}).

// ===== Report an error (Ctrl/Cmd+Shift+E) =====
(function initErrorReport() {
    const modalEl = document.getElementById('error-report-modal');
    if (!modalEl) {
        return;
    }
    const pageUrlInput = document.getElementById('error-report-page-url');
    const selectedInput = document.getElementById('error-report-selected-text');
    const commentInput = document.getElementById('error-report-comment');
    const submitBtn = document.getElementById('error-report-submit');
    const alertEl = document.getElementById('error-report-alert');
    const recaptchaEnabled = Boolean(modalEl.getAttribute('data-recaptcha'));
    let bsModal = null;

    function setAlert(type, message) {
        if (!alertEl) {
            return;
        }
        alertEl.className = 'alert alert-' + type;
        alertEl.textContent = message;
        alertEl.classList.remove('d-none');
    }

    function clearAlert() {
        if (alertEl) {
            alertEl.classList.add('d-none');
            alertEl.textContent = '';
        }
    }

    function getModal() {
        if (!bsModal && typeof bootstrap !== 'undefined' && bootstrap.Modal) {
            bsModal = bootstrap.Modal.getOrCreateInstance(modalEl);
        }
        return bsModal;
    }

    function openWithSelection(text) {
        clearAlert();
        if (pageUrlInput) {
            pageUrlInput.value = window.location.pathname + window.location.search;
        }
        if (selectedInput) {
            selectedInput.value = text;
        }
        if (commentInput) {
            commentInput.value = '';
        }
        const m = getModal();
        if (m) {
            m.show();
        }
    }

    document.addEventListener('keydown', function(e) {
        const key = e.key || e.code;
        if (key !== 'E' && key !== 'KeyE') {
            return;
        }
        if (!e.shiftKey || !(e.ctrlKey || e.metaKey)) {
            return;
        }
        const tag = (e.target && e.target.tagName) ? e.target.tagName.toLowerCase() : '';
        if (tag === 'input' || tag === 'textarea' || tag === 'select' || (e.target && e.target.isContentEditable)) {
            return;
        }
        e.preventDefault();
        const sel = window.getSelection ? window.getSelection().toString().trim() : '';
        if (!sel || sel.length < 3) {
            Kosatka.toast(Kosatka.t('Select a text fragment on the page (at least 3 characters).'), 'warning');
            return;
        }
        openWithSelection(sel);
    });

    function getRecaptchaResponse() {
        if (!recaptchaEnabled || typeof grecaptcha === 'undefined') {
            return '';
        }
        try {
            return grecaptcha.getResponse() || '';
        } catch (err) {
            return '';
        }
    }

    function resetRecaptcha() {
        if (!recaptchaEnabled || typeof grecaptcha === 'undefined') {
            return;
        }
        try {
            grecaptcha.reset();
        } catch (err) {
            /* ignore */
        }
    }

    if (submitBtn) {
        submitBtn.addEventListener('click', function() {
            clearAlert();
            const selected = selectedInput ? selectedInput.value.trim() : '';
            if (selected.length < 3) {
                setAlert('warning', Kosatka.t('The selected text is too short.'));
                return;
            }
            if (recaptchaEnabled && !getRecaptchaResponse()) {
                setAlert('warning', Kosatka.t('Please confirm that you are not a robot.'));
                return;
            }
            submitBtn.disabled = true;
            const payload = {
                selected_text: selected,
                comment: commentInput ? commentInput.value.trim() : '',
                page_url: pageUrlInput ? pageUrlInput.value.trim() : (window.location.pathname + window.location.search),
                'g-recaptcha-response': getRecaptchaResponse()
            };
            fetch('/api/error-report', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                credentials: 'same-origin',
                body: JSON.stringify(payload)
            })
                .then(function(res) {
                    return res.json().then(function(data) {
                        return { ok: res.ok, data: data };
                    });
                })
                .then(function(result) {
                    if (!result.ok) {
                        const msg = (result.data && result.data.error) ? result.data.error : Kosatka.t('Failed to send the message.');
                        setAlert('danger', msg);
                        resetRecaptcha();
                        return;
                    }
                    const m = getModal();
                    if (m) {
                        m.hide();
                    }
                    Kosatka.toast((result.data && result.data.message) ? result.data.message : Kosatka.t('Thank you! Your report has been sent.'), 'success');
                    resetRecaptcha();
                })
                .catch(function() {
                    setAlert('danger', Kosatka.t('Network error. Please try again later.'));
                    resetRecaptcha();
                })
                .finally(function() {
                    submitBtn.disabled = false;
                });
        });
    }
})();
