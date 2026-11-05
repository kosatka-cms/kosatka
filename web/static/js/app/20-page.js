document.addEventListener('DOMContentLoaded', function() {

    // ===== Admin bar: collapse / expand =====
    (function initAdminTopBar() {
        const STORAGE_KEY = 'cmsAdminTopBarHidden';
        const bar = document.getElementById('admin-top-bar');
        const collapseBtn = document.getElementById('admin-top-bar-collapse');
        const restoreBtn = document.getElementById('admin-top-bar-restore');
        if (!bar && !restoreBtn) {
            return;
        }
        function applyFromStorage() {
            try {
                if (localStorage.getItem(STORAGE_KEY) === '1') {
                    document.body.classList.add('admin-top-bar-hidden');
                } else {
                    document.body.classList.remove('admin-top-bar-hidden');
                }
            } catch (e) {
                /* private mode / blocked storage */
            }
        }
        function setHidden(hidden) {
            try {
                if (hidden) {
                    localStorage.setItem(STORAGE_KEY, '1');
                } else {
                    localStorage.removeItem(STORAGE_KEY);
                }
            } catch (e) {
                /* private mode / blocked storage */
            }
            if (hidden) {
                document.body.classList.add('admin-top-bar-hidden');
            } else {
                document.body.classList.remove('admin-top-bar-hidden');
            }
        }
        // --admin-bar-offset - the fixed bar's height (0 when collapsed):
        // the room left for it at the top of the page (.admin-top-bar-spacer)
        // and where sticky elements stop. A submenu open on a phone lies
        // over the page instead of pushing it down: the height then stays.
        function syncOffset() {
            const shown = bar && !document.body.classList.contains('admin-top-bar-hidden');
            if (shown && bar.querySelector('.admin-bar-dropdown.is-open')) {
                return;
            }
            document.documentElement.style.setProperty('--admin-bar-offset', (shown ? bar.offsetHeight : 0) + 'px');
        }
        new MutationObserver(syncOffset).observe(document.body, { attributes: true, attributeFilter: ['class'] });
        window.addEventListener('resize', syncOffset);
        // The bar's own height changes too: it wraps, a notice comes.
        if (bar && window.ResizeObserver) {
            new ResizeObserver(syncOffset).observe(bar);
        }
        applyFromStorage();
        syncOffset();
        if (collapseBtn) {
            collapseBtn.addEventListener('click', function() {
                setHidden(true);
            });
        }
        if (restoreBtn) {
            restoreBtn.addEventListener('click', function() {
                setHidden(false);
            });
        }
    })();

    // ===== Admin bar: dropdowns open on tap only (narrow screens) =====
    (function initAdminBarDropdowns() {
        const bar = document.getElementById('admin-top-bar');
        if (!bar) {
            return;
        }
        function isMobileDropdownMode() {
            return window.matchMedia('(max-width: 768px)').matches;
        }
        // The "Menu" button of a phone's bar: the sections over the page.
        const menuToggle = bar.querySelector('.admin-bar-menu-toggle');
        function setMenuOpen(open) {
            bar.classList.toggle('admin-bar-menu-open', open);
            if (menuToggle) {
                menuToggle.setAttribute('aria-expanded', open ? 'true' : 'false');
            }
        }
        function closeDropdowns(except) {
            bar.querySelectorAll('.admin-bar-dropdown.is-open').forEach(function (el) {
                if (el === except) {
                    return;
                }
                el.classList.remove('is-open');
                const bt = el.querySelector(':scope > .admin-bar-toggle');
                if (bt) {
                    bt.setAttribute('aria-expanded', 'false');
                }
            });
        }
        function clearOpenState() {
            closeDropdowns(null);
            setMenuOpen(false);
        }
        if (menuToggle) {
            menuToggle.addEventListener('click', function (e) {
                e.preventDefault();
                e.stopPropagation();
                if (bar.classList.contains('admin-bar-menu-open')) {
                    clearOpenState();
                } else {
                    closeDropdowns(null);
                    setMenuOpen(true);
                }
            });
        }
        document.addEventListener('keydown', function (e) {
            if (e.key === 'Escape' && bar.classList.contains('admin-bar-menu-open')) {
                clearOpenState();
                if (menuToggle) {
                    menuToggle.focus();
                }
            }
        });
        try {
            const mq = window.matchMedia('(max-width: 768px)');
            const onMqChange = function () {
                if (!mq.matches) {
                    clearOpenState();
                }
            };
            if (mq.addEventListener) {
                mq.addEventListener('change', onMqChange);
            } else if (mq.addListener) {
                mq.addListener(onMqChange);
            }
        } catch (e) {
            /* noop */
        }
        bar.addEventListener('click', function (e) {
            if (!isMobileDropdownMode()) {
                return;
            }
            const toggle = e.target.closest('.admin-bar-toggle');
            if (!toggle || !bar.contains(toggle)) {
                return;
            }
            e.preventDefault();
            e.stopPropagation();
            const wrap = toggle.closest('.admin-bar-dropdown');
            if (!wrap) {
                return;
            }
            const opening = !wrap.classList.contains('is-open');
            if (opening) {
                // One thing open at a time: "Create" or the user's menu
                // closes the sections, and the other way round.
                closeDropdowns(wrap);
                if (!wrap.closest('.admin-bar-sections')) {
                    setMenuOpen(false);
                }
                wrap.classList.add('is-open');
                toggle.setAttribute('aria-expanded', 'true');
            } else {
                wrap.classList.remove('is-open');
                toggle.setAttribute('aria-expanded', 'false');
            }
        });
        document.addEventListener(
            'click',
            function (e) {
                if (!isMobileDropdownMode()) {
                    return;
                }
                if (bar.contains(e.target)) {
                    return;
                }
                clearOpenState();
            },
            true
        );
    })();

    // ===== Main menu =====
    const menuIcon = document.getElementById('menu-icon');
    const navMenu = document.querySelector('#main-menu ul.menu.ul-depth-0');
    const navWrap = document.getElementById('nav-wrap');

    /* The mobile panel is hidden via CSS (none !important); on desktop drop the inline style so the min-width media query applies */
    if (navMenu) {
        if (window.innerWidth > 768) {
            navMenu.style.removeProperty('display');
        } else {
            setMainNavMobileOpen(navMenu, false);
        }
    }

    if (menuIcon && navMenu) {
        menuIcon.addEventListener('click', function(e) {
            e.preventDefault();
            e.stopPropagation();

            this.classList.toggle('active');
            setMainNavMobileOpen(navMenu, this.classList.contains('active'));
        });
    }

    // Close the menu on an outside click (mobile only)
    document.addEventListener('click', function(e) {
        if (window.innerWidth <= 768) {
            if (navWrap && !navWrap.contains(e.target)) {
                if (navMenu) {
                    setMainNavMobileOpen(navMenu, false);
                }
                if (menuIcon) {
                    menuIcon.classList.remove('active');
                }
            }
        }
    });

    // Don't let clicks inside the menu bubble up
    if (navWrap) {
        navWrap.addEventListener('click', function(e) {
            e.stopPropagation();
        });
    }

    // On a resize: a wide window gets the desktop menu back, a narrow one
    // the mobile menu closed.
    window.addEventListener('resize', function() {
        if (navMenu) {
            if (window.innerWidth > 768) {
                navMenu.style.removeProperty('display');
                if (menuIcon) {
                    menuIcon.classList.remove('active');
                }
            } else {
                setMainNavMobileOpen(navMenu, false);
                if (menuIcon) {
                    menuIcon.classList.remove('active');
                }
            }
        }
    });

    // On mobile, main menu submenus are always expanded (menu.css:
    // ul.ul-depth-0 ul { display: block }); there is deliberately no accordion.

    // ===== User menu =====

    // The account block in the mobile menu (initUserMenu).
    initUserMenu();
    if (navMenu && window.innerWidth <= 768) {
        setMainNavMobileOpen(navMenu, false);
        if (menuIcon) {
            menuIcon.classList.remove('active');
        }
    }

    // Desktop dropdown
    const userMenuToggle = document.getElementById('user-menu-toggle');
    const userDropdownMenu = document.getElementById('user-dropdown-menu');

    if (userMenuToggle && userDropdownMenu) {
        userMenuToggle.addEventListener('click', function(e) {
            e.preventDefault();
            e.stopPropagation();

            this.classList.toggle('active');
            userDropdownMenu.classList.toggle('show');
            if (!userDropdownMenu.classList.contains('show')) {
                const createRoot = document.querySelector('.user-menu-dropdown .dropdown-create-submenu');
                const createBtn = document.getElementById('user-create-submenu-toggle');
                if (createRoot) {
                    createRoot.classList.remove('is-open');
                }
                if (createBtn) {
                    createBtn.setAttribute('aria-expanded', 'false');
                }
            }
        });

        // Close the dropdown on an outside click
        document.addEventListener('click', function(e) {
            if (!e.target.closest('.user-menu-dropdown')) {
                userMenuToggle.classList.remove('active');
                userDropdownMenu.classList.remove('show');
                const createRoot = document.querySelector('.user-menu-dropdown .dropdown-create-submenu');
                const createBtn = document.getElementById('user-create-submenu-toggle');
                if (createRoot) {
                    createRoot.classList.remove('is-open');
                }
                if (createBtn) {
                    createBtn.setAttribute('aria-expanded', 'false');
                }
            }
        });

        const createToggle = document.getElementById('user-create-submenu-toggle');
        const createSubmenuRoot = createToggle ? createToggle.closest('.dropdown-create-submenu') : null;
        if (createToggle && createSubmenuRoot) {
            createToggle.addEventListener('click', function(e) {
                e.preventDefault();
                e.stopPropagation();
                const open = createSubmenuRoot.classList.toggle('is-open');
                createToggle.setAttribute('aria-expanded', open ? 'true' : 'false');
            });
            const createItems = document.getElementById('user-create-submenu');
            if (createItems) {
                createItems.querySelectorAll('a').forEach(function(a) {
                    a.addEventListener('click', function() {
                        userMenuToggle.classList.remove('active');
                        userDropdownMenu.classList.remove('show');
                        createSubmenuRoot.classList.remove('is-open');
                        createToggle.setAttribute('aria-expanded', 'false');
                    });
                });
            }
        }
    }

    // Logout: one delegated listener, for the menus built later (the
    // mobile one) too - a link handled twice would log out twice.
    document.addEventListener('click', function(e) {
        if (e.target.closest('#logout-link, .mobile-logout-link, .admin-bar-logout')) {
            e.preventDefault();
            handleLogout();
        }
    });

    initLiveSearch();
    initBackToTop();

});

