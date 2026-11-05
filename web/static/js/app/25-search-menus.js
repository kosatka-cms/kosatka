// Live search (debounced, results grouped by section), the back-to-top
// button and the mobile user menu.
function initLiveSearch() {
    const searchInput = document.getElementById('search-main');
    const resultsContainer = document.querySelector('.search_autocomplete_results');
    const searchBlock = document.querySelector('.block-search');
    const searchForm = document.getElementById('siteSearch');

    if (!searchInput || !resultsContainer) return;

    // A form without an action (a theme's block without a results page)
    // stays on the page: submitting shows the live results.
    if (searchForm && !searchForm.getAttribute('action')) {
        searchForm.addEventListener('submit', function (e) {
            e.preventDefault();
            searchInput.dispatchEvent(new Event('input'));
        });
    }

    let debounceTimer = null;
    let searchAbortController = null;
    const DEBOUNCE_MS = 250;
    const MIN_CHARS = 3;
    // The search menu's links (Search settings), shown while the query is
    // shorter than MIN_CHARS: fetched once, null - not yet.
    let categories = null;

    function abortPendingSearch() {
        if (searchAbortController) {
            searchAbortController.abort();
            searchAbortController = null;
        }
    }

    function renderResults(results) {
        if (!results || results.length === 0) {
            resultsContainer.innerHTML = '<li class="item_result item_empty"><span class="search_title">' + Kosatka.escapeHtml(Kosatka.t('Nothing found')) + '</span></li>';
            resultsContainer.style.display = 'block';
            return;
        }
        resultsContainer.innerHTML = results.map(function(r) {
            const subtitle = r.subtitle ? '<div class="search_subtitle">' + Kosatka.escapeHtml(r.subtitle) + '</div>' : '';
            // "Section / Article": the badge's terms, then the type.
            const badge = r.badge ? '<span class="search-add search-badge">' + Kosatka.escapeHtml(r.badge) + (r.type_label ? ' / ' : '') + '</span>' : '';
            const typeLabel = r.type_label ? '<span class="search-add search-type">' + Kosatka.escapeHtml(r.type_label) + '</span>' : '';
            return '<li class="item_result" data-id="' + r.id + '" data-url="' + Kosatka.escapeHtml(r.url) + '">' +
                '<a href="' + Kosatka.escapeHtml(r.url) + '">' +
                '<img src="' + Kosatka.escapeHtml(r.image_url) + '" class="search_img" alt="">' +
                '<div class="search_result_text">' +
                '<div class="search_title">' + Kosatka.escapeHtml(r.title) + '</div>' +
                subtitle +
                badge +
                typeLabel +
                '</div></a></li>';
        }).join('');
        resultsContainer.style.display = 'block';
    }

    function renderCategories(list) {
        if (!list || list.length === 0) {
            resultsContainer.innerHTML = '';
            resultsContainer.style.display = 'none';
            return;
        }
        resultsContainer.innerHTML = list.map(function(c) {
            return '<li class="item_result item_category"><a href="' + Kosatka.escapeHtml(c.url) + '">' +
                '<span class="search_title">' + Kosatka.escapeHtml(c.label) + '</span></a></li>';
        }).join('');
        resultsContainer.style.display = 'block';
    }

    // showCategories - the search menu's links in place of results (the
    // query too short to search).
    function showCategories() {
        abortPendingSearch();
        if (categories !== null) {
            renderCategories(categories);
            return;
        }
        fetch('/api/search?q=')
            .then(function(res) { return res.json(); })
            .then(function(data) {
                categories = data.categories || [];
                if (searchInput.value.trim().length < MIN_CHARS) {
                    renderCategories(categories);
                }
            })
            .catch(function() { categories = []; });
    }

    function showLoading() {
        resultsContainer.innerHTML = '<li class="search-loading"><span class="search-spinner"></span><span class="search-loading-text">' + Kosatka.escapeHtml(Kosatka.t('Searching...')) + '</span></li>';
        resultsContainer.style.display = 'block';
    }

    function fetchSearch(query) {
        abortPendingSearch();
        searchAbortController = new AbortController();
        const signal = searchAbortController.signal;
        showLoading();
        const url = '/api/search?q=' + encodeURIComponent(query);
        fetch(url, { signal: signal })
            .then(function(res) { return res.json(); })
            .then(function(data) {
                if (data.results) {
                    renderResults(data.results);
                } else {
                    resultsContainer.innerHTML = '';
                    resultsContainer.style.display = 'none';
                }
            })
            .catch(function(err) {
                if (err && err.name === 'AbortError') {
                    return;
                }
                console.error('Search error:', err);
                resultsContainer.innerHTML = '<li class="item_result item_empty"><span class="search_title">' + Kosatka.escapeHtml(Kosatka.t('Search error')) + '</span></li>';
                resultsContainer.style.display = 'block';
            });
    }

    function onSearchInput() {
        clearTimeout(debounceTimer);
        const query = searchInput.value.trim();
        debounceTimer = setTimeout(function() {
            if (query.length < MIN_CHARS) {
                showCategories();
            } else {
                fetchSearch(query);
            }
        }, DEBOUNCE_MS);
    }

    searchInput.addEventListener('input', onSearchInput);
    searchInput.addEventListener('focus', function() {
        const query = searchInput.value.trim();
        if (query.length >= MIN_CHARS) {
            fetchSearch(query);
        } else {
            showCategories();
        }
    });

    // Hide on an outside click (delayed so a result link can be followed)
    document.addEventListener('click', function(e) {
        if (searchBlock && !searchBlock.contains(e.target)) {
            setTimeout(function() {
                resultsContainer.style.display = 'none';
            }, 150);
        }
    });

    resultsContainer.style.display = 'none';
}

function initBackToTop() {
    const btn = document.getElementById('back-to-top');
    if (!btn) return;

    const SCROLL_THRESHOLD = 300;

    function toggleVisibility() {
        if (window.scrollY > SCROLL_THRESHOLD) {
            btn.classList.add('visible');
        } else {
            btn.classList.remove('visible');
        }
    }

    window.addEventListener('scroll', toggleVisibility, { passive: true });
    toggleVisibility();

    btn.addEventListener('click', function() {
        window.scrollTo({ top: 0, behavior: 'smooth' });
    });
}

// initUserMenu builds the mobile menu (the desktop one, block_user_menu,
// is rendered for the visitor by the server).
function initUserMenu() {
    if (USER_CONFIG.isLoggedIn) {
        createMobileUserMenu();
    } else {
        createMobileGuestMenu();
    }
}

function createMobileUserMenu() {
    const navMenu = document.querySelector('#main-menu ul.menu.ul-depth-0');
    if (!navMenu) return;

    if (document.querySelector('.mobile-user-menu')) return;

    // "Create content" - a button folding out the types the user may
    // create (folded at first), as in the desktop user menu; none - no
    // item.
    const materials = Array.isArray(USER_CONFIG.creatableMaterials) ? USER_CONFIG.creatableMaterials : [];
    let createSection = '';
    if (materials.length > 0) {
        const sublinks = materials.map(function(m) {
            const url = Kosatka.escapeHtml(m.url);
            const label = Kosatka.escapeHtml(m.label);
            return '<a href="' + url + '" class="dropdown-item dropdown-subitem">' + label + '</a>';
        }).join('');
        const createLabel = Kosatka.escapeHtml(Kosatka.t('Create content'));
        createSection = `
        <div class="mobile-create-submenu">
            <button type="button" class="mobile-user-menu-action mobile-create-toggle" aria-expanded="false" aria-controls="mobile-create-items">
                <span class="mobile-create-toggle-main">
                    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
                        <path d="M8 4a.5.5 0 0 1 .5.5v3h3a.5.5 0 0 1 0 1h-3v3a.5.5 0 0 1-1 0v-3h-3a.5.5 0 0 1 0-1h3v-3A.5.5 0 0 1 8 4z"/>
                    </svg>
                    ${createLabel}
                </span>
                <svg class="mobile-create-chevron" width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
                    <path d="M3 4.5L6 7.5L9 4.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
            </button>
            <div class="mobile-create-items" id="mobile-create-items">${sublinks}</div>
        </div>`;
    }

    const displayName = Kosatka.escapeHtml(USER_CONFIG.username);
    const avatarUrl = Kosatka.escapeHtml(USER_CONFIG.avatar);

    // The modules' items of the desktop user menu marked
    // data-mobile-user-menu (their "user_menu_<module>" templates, e.g.
    // the messages): copied, after the profile.
    const moduleItems = Array.prototype.map.call(
        document.querySelectorAll('#user-dropdown-menu [data-mobile-user-menu]'),
        function(item) {
            const copy = item.cloneNode(true);
            copy.removeAttribute('id');
            copy.className = 'dropdown-item mobile-user-menu-action';
            return copy.outerHTML;
        }
    ).join('');

    const mobileUserMenu = document.createElement('li');
    mobileUserMenu.className = 'mobile-user-menu';
    mobileUserMenu.innerHTML = `
        <div class="mobile-user-menu-panel">
            <div class="mobile-user-menu-label">${Kosatka.escapeHtml(Kosatka.t('Account'))}</div>
            <div class="user-info">
                <a href="/me" class="user-info-profile-link" aria-label="${Kosatka.escapeHtml(Kosatka.t('My profile'))}">
                    <img src="${avatarUrl}" alt="" class="user-avatar-small">
                    <span class="user-name">${displayName}</span>
                </a>
                <a href="/logout" class="mobile-logout-link user-info-logout">${Kosatka.escapeHtml(Kosatka.t('Log out'))}</a>
            </div>
        ${moduleItems}
        ${createSection}
        </div>
    `;

    navMenu.insertBefore(mobileUserMenu, navMenu.firstChild);

    const createToggle = mobileUserMenu.querySelector('.mobile-create-toggle');
    if (createToggle) {
        createToggle.addEventListener('click', function() {
            const open = createToggle.parentElement.classList.toggle('is-open');
            createToggle.setAttribute('aria-expanded', open ? 'true' : 'false');
        });
    }
}

function createMobileGuestMenu() {
    const navMenu = document.querySelector('#main-menu ul.menu.ul-depth-0');
    if (!navMenu) return;

    if (document.querySelector('.mobile-user-menu')) return;

    // "Register" as the page has it: a closed registration (account
    // settings) has no such link.
    const register = document.querySelector('a[href="/user/register"]')
        ? `<a href="/user/register" class="user-menu-link">${Kosatka.escapeHtml(Kosatka.t('Register'))}</a>`
        : '';
    const mobileUserMenu = document.createElement('li');
    mobileUserMenu.className = 'mobile-user-menu';
    mobileUserMenu.innerHTML = `
        <div class="mobile-user-menu-panel">
            <div class="mobile-user-menu-label">${Kosatka.escapeHtml(Kosatka.t('Account'))}</div>
            <div class="user-menu-guest">
                <a href="/login" class="user-menu-link">${Kosatka.escapeHtml(Kosatka.t('Log in'))}</a>
                ${register}
            </div>
        </div>
    `;

    navMenu.insertBefore(mobileUserMenu, navMenu.firstChild);
}

// Logs out: POSTs /logout (the fetch wrapper in csrf_head adds the CSRF
// token), then goes to the front page; on failure - to GET /logout, the
// page with the logout button.
function handleLogout() {
    fetch('/logout', {method: 'POST', credentials: 'same-origin'})
        .then(function (response) {
            window.location.href = response.ok ? '/' : '/logout';
        })
        .catch(function () {
            window.location.href = '/logout';
        });
}

