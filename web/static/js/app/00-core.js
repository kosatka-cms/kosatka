
// User data rendered by the backend (window.USER_DATA), with guest defaults.
const USER_CONFIG = window.USER_DATA || {
    isLoggedIn: false,
    username: 'Username',
    avatar: '/static/images/no_avatar.png', // entity.AvatarPlaceholderURL
    creatableMaterials: []
};

// ===== Behaviors =====
// Kosatka.behaviors - the scripts that bring a piece of markup to life (a
// carousel, a lazy block, a module's widget). Each
// is { attach: function (context) { ... } }: Kosatka.attachBehaviors calls
// every one for the page once it has loaded, and again for each piece of
// HTML put in the page later (a lazy block, an AJAX page of a list, the
// comments, a module's fragment) with that piece as the context. attach
// looks only inside context and must be safe to run again on markup it has
// seen: Kosatka.once hands it each element only once. Register behaviors
// before DOMContentLoaded (app.js, a module's or theme's deferred script);
// code that puts HTML in the page calls Kosatka.attachBehaviors(element).
// docs/themes/scripts.md.
window.Kosatka = window.Kosatka || {};
Kosatka.behaviors = Kosatka.behaviors || {};

Kosatka.attachBehaviors = function (context) {
    context = context || document;
    Object.keys(Kosatka.behaviors).forEach(function (name) {
        var behavior = Kosatka.behaviors[name];
        if (!behavior || typeof behavior.attach !== 'function') {
            return;
        }
        // One behavior failing doesn't stop the others.
        try {
            behavior.attach(context);
        } catch (err) {
            console.error('Kosatka behavior ' + name + ':', err);
        }
    });
};

// Kosatka.once - the elements matching selector in context (context
// itself included) not yet handled under id; marks them handled (the
// data-once attribute lists the ids).
Kosatka.once = function (id, selector, context) {
    context = context || document;
    var found = [];
    if (context.nodeType === 1 && context.matches(selector)) {
        found.push(context);
    }
    found = found.concat(Array.prototype.slice.call(context.querySelectorAll(selector)));
    return found.filter(function (el) {
        var done = (el.getAttribute('data-once') || '').split(' ').filter(Boolean);
        if (done.indexOf(id) !== -1) {
            return false;
        }
        done.push(id);
        el.setAttribute('data-once', done.join(' '));
        return true;
    });
};

document.addEventListener('DOMContentLoaded', function () {
    Kosatka.attachBehaviors(document);
});

function setMainNavMobileOpen(navMenu, open) {
    if (!navMenu) {
        return;
    }
    if (open) {
        navMenu.style.setProperty('display', 'block', 'important');
    } else {
        navMenu.style.setProperty('display', 'none', 'important');
    }
}

// Kosatka.escapeHtml - s as HTML text or an attribute value (& < > " '
// escaped); null and undefined are "". For HTML built in strings by the
// engine's, modules' and themes' scripts.
Kosatka.escapeHtml = function (s) {
    if (s == null) {
        return '';
    }
    return String(s).replace(/[&<>"']/g, function (c) {
        return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
};

