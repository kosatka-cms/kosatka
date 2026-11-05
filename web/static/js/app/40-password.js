// ===== Password strength =====
// passwordStrength - [data-password-strength="<input id>"]
// (partials/password_strength.html): each li[data-password-rule] gets
// "is-met" while the password meets it - length (data-min-length),
// lower, upper, digit, punct; letters and digits of any script.
var passwordRules = {
    // Characters, as the server counts them (not UTF-16 units).
    length: function (v, li) { return Array.from(v).length >= (parseInt(li.getAttribute('data-min-length'), 10) || 0); },
    lower: function (v) { return /\p{Ll}/u.test(v); },
    upper: function (v) { return /\p{Lu}/u.test(v); },
    digit: function (v) { return /\p{Nd}/u.test(v); },
    punct: function (v) { return /[^\p{L}\p{N}\s]/u.test(v); }
};

Kosatka.behaviors.passwordStrength = {
    attach: function (context) {
        Kosatka.once('password-strength', '[data-password-strength]', context).forEach(function (list) {
            var input = document.getElementById(list.getAttribute('data-password-strength'));
            if (!input) return;
            input.addEventListener('input', function () {
                list.querySelectorAll('li[data-password-rule]').forEach(function (li) {
                    var rule = passwordRules[li.getAttribute('data-password-rule')];
                    li.classList.toggle('is-met', !!rule && rule(input.value, li));
                });
            });
        });
    }
};

