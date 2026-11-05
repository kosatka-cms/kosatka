// ===== Comment form =====
// The comment form of a page (node comments, a forum topic): a hidden
// [data-comment-form] with input[name=parent_id], a textarea with
// data-editor-config, [data-reply-info] holding [data-reply-author], and
// [data-cancel-reply] buttons. A link with data-comment-open shows it
// (with its editor, loaded then); a link with data-reply-to="<comment
// id>" and data-author also makes the comment a reply; Cancel turns the
// reply back into a comment and empties the editor; #comment-form in the
// address opens the form. The links may come later (an AJAX page of
// comments), so the clicks are caught on the document.
function commentForm() {
    return document.querySelector('[data-comment-form]');
}

function openCommentForm(form) {
    form.style.display = 'block';
    form.scrollIntoView({ behavior: 'smooth' });
    var textarea = form.querySelector('textarea[data-editor-config]');
    if (textarea) {
        Kosatka.loadEditor(textarea).then(function (editor) { editor.focus(); });
    }
}

function cancelCommentReply(form) {
    var parent = form.querySelector('input[name="parent_id"]');
    var info = form.querySelector('[data-reply-info]');
    if (parent) parent.value = '0';
    if (info) info.style.display = 'none';
    var textarea = form.querySelector('textarea[data-editor-config]');
    var editor = textarea && window.tinymce ? tinymce.get(textarea.id) : null;
    if (editor) editor.setContent('');
}

document.addEventListener('click', function (e) {
    var form = commentForm();
    if (!form) return;
    var cancel = e.target.closest('[data-cancel-reply]');
    if (cancel && form.contains(cancel)) {
        e.preventDefault();
        cancelCommentReply(form);
        return;
    }
    var reply = e.target.closest('[data-reply-to]');
    if (reply) {
        e.preventDefault();
        var parent = form.querySelector('input[name="parent_id"]');
        var info = form.querySelector('[data-reply-info]');
        if (parent) parent.value = reply.getAttribute('data-reply-to');
        if (info) {
            var author = info.querySelector('[data-reply-author]');
            if (author) author.textContent = reply.getAttribute('data-author') || '';
            info.style.display = 'block';
        }
        openCommentForm(form);
        return;
    }
    if (e.target.closest('[data-comment-open]')) {
        e.preventDefault();
        openCommentForm(form);
    }
});

document.addEventListener('DOMContentLoaded', function () {
    var form = commentForm();
    if (form && window.location.hash === '#comment-form') {
        openCommentForm(form);
    }
});

