// ===== Comments =====
// comments - a content page's comments (#comments, data-node-id) load from
// /api/comments/<node>?page=N once the section nears the viewport (right
// away without IntersectionObserver, at once for a link to a comment) -
// unless the page has them already (data-loaded: the comment settings'
// "with the page").
// commentPager swaps a page of them in
// place; Reply opens the comment form (COMMENT FORM below).
Kosatka.behaviors.comments = {
    attach: function (context) {
        Kosatka.once('comments', '#comments[data-node-id]:not([data-loaded])', context).forEach(function (section) {
            const nodeId = section.dataset.nodeId;
            // A link to a comment (<node>?page=N#comment-ID, where the
            // comment redirect sends) loads that page of comments right
            // away and scrolls to the comment.
            if (/^#comment-\d+$/.test(window.location.hash)) {
                const page = parseInt(new URLSearchParams(window.location.search).get('page'), 10) || 1;
                loadComments(section, nodeId, page).then(function () {
                    const target = document.getElementById(window.location.hash.slice(1));
                    if (target) target.scrollIntoView({ block: 'start' });
                });
                return;
            }
            if (!('IntersectionObserver' in window)) {
                loadComments(section, nodeId, 1);
                return;
            }
            const observer = new IntersectionObserver((entries) => {
                if (entries.some((entry) => entry.isIntersecting)) {
                    observer.disconnect();
                    loadComments(section, nodeId, 1);
                }
            }, { threshold: 0.1, rootMargin: '50px' });
            observer.observe(section);
        });
    }
};

Kosatka.behaviors.commentPager = {
    attach: function (context) {
        Kosatka.once('comment-pager', '#comments .pagination', context).forEach(function (pagination) {
            const section = pagination.closest('#comments');
            pagination.addEventListener('click', function (e) {
                const link = e.target.closest('a');
                if (!link) return;
                e.preventDefault();
                const page = new URL(link.href, window.location.origin).searchParams.get('page') || 1;
                loadCommentsPage(section, section.dataset.nodeId, page);
            });
        });
    }
};

async function loadComments(section, nodeId, page) {
    const loadingElement = section.querySelector('.comments-loading');
    const contentElement = section.querySelector('.comments-content');
    if (!contentElement) {
        return;
    }
    if (loadingElement) {
        loadingElement.style.display = 'block';
    }
    try {
        const response = await fetch(`/api/comments/${nodeId}?page=${page}`);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        contentElement.innerHTML = data.html;
        section.classList.add('comments-loaded');
        Kosatka.attachBehaviors(contentElement);

        // Hide the comments section while it is empty, if the content type says so
        // (content_types.hide_empty_comments).
        if (section.dataset.hideEmpty === '1' && !contentElement.querySelector('.message')) {
            section.style.display = 'none';
            const commentForm = document.getElementById('comment-form');
            const authNoticeBottom = document.querySelector('.auth-notice-bottom');
            if (commentForm) commentForm.style.display = 'none';
            if (authNoticeBottom) authNoticeBottom.style.display = 'none';
        }
    } catch (error) {
        console.error('Error loading comments:', error);
        contentElement.innerHTML = '<p style="color: #d32f2f; padding: 20px;">' + Kosatka.escapeHtml(Kosatka.t('Failed to load comments. Please refresh the page.')) + '</p>';
    } finally {
        if (loadingElement) {
            loadingElement.style.display = 'none';
        }
    }
}

async function loadCommentsPage(section, nodeId, page) {
    const contentElement = section.querySelector('.comments-content');
    if (!contentElement) return;
    contentElement.style.opacity = '0.5';
    contentElement.style.pointerEvents = 'none';
    try {
        const response = await fetch(`/api/comments/${nodeId}?page=${page}`);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        contentElement.innerHTML = data.html;
        Kosatka.attachBehaviors(contentElement);
        section.scrollIntoView({ behavior: 'smooth', block: 'start' });
    } catch (error) {
        console.error('Error loading comments page:', error);
        Kosatka.alert(Kosatka.t('Failed to load comments. Please try again.'));
    } finally {
        contentElement.style.opacity = '1';
        contentElement.style.pointerEvents = 'auto';
    }
}

