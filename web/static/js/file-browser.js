/**
 * File browser for a user's files (/api/user/files): folders, upload
 * (button or drag and drop), rename, delete, copy link, preview.
 * Used as a page (/me/files, admin) and as a picker (TinyMCE dialogs).
 *
 *   KosatkaFileBrowser.mount(el, {
 *     mode: 'page' | 'picker',
 *     accept: 'any' | 'image',   // picker: what can be selected
 *     userID: 0,                 // another user's files (admin)
 *     formatId: 0,               // text format for the image upload policy
 *     path: '',                  // initial folder
 *     csrfToken: '',
 *     rootLabel: '',             // name of the top folder ("My files")
 *     onSelect: function (url, entry) {},
 *     onPathChange: function (path) {},
 *   })
 */
(function (global) {
  'use strict';

  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text !== undefined) e.textContent = text;
    return e;
  }

  function button(cls, text, title) {
    var b = el('button', 'kfb-btn ' + (cls || ''), text);
    b.type = 'button';
    if (title) b.title = title;
    return b;
  }

  function formatSize(n) {
    if (n === null || n === undefined) return '';
    if (n < 1024) return Kosatka.t('@size B', { '@size': n });
    if (n < 1048576) return Kosatka.t('@size KB', { '@size': (n / 1024).toFixed(1) });
    return Kosatka.t('@size MB', { '@size': (n / 1048576).toFixed(1) });
  }

  function formatDate(iso) {
    if (!iso) return '';
    try {
      return new Date(iso).toLocaleString(global.Kosatka ? Kosatka.lang : undefined, {
        year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
      });
    } catch (e) {
      return iso;
    }
  }

  function extOf(name) {
    var i = name.lastIndexOf('.');
    return i > 0 ? name.slice(i + 1).toUpperCase() : '';
  }

  function joinPath(dir, name) {
    return dir ? dir + '/' + name : name;
  }

  function mount(host, opts) {
    opts = opts || {};
    var mode = opts.mode === 'picker' ? 'picker' : 'page';
    var accept = opts.accept === 'image' ? 'image' : 'any';
    var rootLabel = opts.rootLabel || Kosatka.t('My files');
    var state = {
      path: opts.path || '',
      tree: [],
      entries: [],
      baseURL: '',
      view: 'list',
      filter: '',
    };
    try {
      state.view = global.localStorage.getItem('kfbView') === 'grid' ? 'grid' : 'list';
    } catch (e) { /* storage unavailable */ }

    function csrf() {
      if (opts.csrfToken) return opts.csrfToken;
      var m = document.querySelector('meta[name="csrf-token"]');
      return m ? m.getAttribute('content') : '';
    }

    function apiURL(path, params) {
      var q = [];
      params = params || {};
      if (opts.userID) params.user = opts.userID;
      Object.keys(params).forEach(function (k) {
        if (params[k] !== '' && params[k] !== undefined && params[k] !== null) {
          q.push(encodeURIComponent(k) + '=' + encodeURIComponent(params[k]));
        }
      });
      return '/api/user/files' + path + (q.length ? '?' + q.join('&') : '');
    }

    function request(method, path, params, body) {
      return fetch(apiURL(path, method === 'GET' ? params : null), {
        method: method,
        body: body,
        credentials: 'same-origin',
      }).then(function (r) {
        return r.json().catch(function () { return {}; }).then(function (j) {
          if (!r.ok) throw new Error(j.error || Kosatka.t('Error'));
          return j;
        });
      });
    }

    function post(path, fields) {
      var fd = new FormData();
      fd.append('csrf_token', csrf());
      Object.keys(fields).forEach(function (k) { fd.append(k, fields[k]); });
      return request('POST', path, null, fd);
    }

    // Layout.
    host.innerHTML = '';
    host.classList.add('kfb', 'kfb--' + mode);
    var toolbar = el('div', 'kfb-toolbar');
    var crumbs = el('div', 'kfb-crumbs');
    var actions = el('div', 'kfb-actions');
    var uploadBtn = button('kfb-btn-primary', Kosatka.t('Upload'), Kosatka.t('Upload to the current folder'));
    var mkdirBtn = button('', Kosatka.t('New folder'));
    var search = el('input', 'kfb-search');
    search.type = 'search';
    search.placeholder = Kosatka.t('Search in folder');
    var viewBtn = button('kfb-view', '', Kosatka.t('Switch view'));
    var fileInput = el('input');
    fileInput.type = 'file';
    fileInput.multiple = true;
    fileInput.hidden = true;
    var imageTypes = 'image/jpeg,image/png,image/gif,image/webp';
    if (accept === 'image') fileInput.accept = imageTypes;
    actions.appendChild(search);
    actions.appendChild(viewBtn);
    actions.appendChild(mkdirBtn);
    actions.appendChild(uploadBtn);
    actions.appendChild(fileInput);
    toolbar.appendChild(crumbs);
    toolbar.appendChild(actions);

    var status = el('div', 'kfb-status');
    status.setAttribute('role', 'status');

    var body = el('div', 'kfb-body');
    var treeCol = el('nav', 'kfb-tree');
    treeCol.setAttribute('aria-label', Kosatka.t('Folders'));
    var main = el('div', 'kfb-main');
    var list = el('div', 'kfb-list');
    var drop = el('div', 'kfb-drop', Kosatka.t('Drop files here to upload them to this folder'));
    main.appendChild(list);
    main.appendChild(drop);
    body.appendChild(treeCol);
    body.appendChild(main);

    host.appendChild(toolbar);
    host.appendChild(status);
    host.appendChild(body);

    function setStatus(text, isError) {
      status.textContent = text || '';
      status.classList.toggle('kfb-status--error', !!isError);
    }

    function fail(err) {
      setStatus((err && err.message) || String(err), true);
    }

    // Breadcrumbs.
    function renderCrumbs() {
      crumbs.innerHTML = '';
      var root = button('kfb-crumb', rootLabel);
      root.onclick = function () { go(''); };
      crumbs.appendChild(root);
      var acc = '';
      state.path.split('/').filter(Boolean).forEach(function (part) {
        acc = joinPath(acc, part);
        crumbs.appendChild(el('span', 'kfb-crumb-sep', '/'));
        var target = acc;
        var b = button('kfb-crumb', part);
        b.onclick = function () { go(target); };
        crumbs.appendChild(b);
      });
    }

    // Folder tree.
    function renderTree() {
      treeCol.innerHTML = '';
      var ul = el('ul');
      var rootLi = el('li');
      var rootBtn = button('kfb-tree-item' + (state.path === '' ? ' is-active' : ''), rootLabel);
      rootBtn.onclick = function () { go(''); };
      rootLi.appendChild(rootBtn);
      ul.appendChild(rootLi);
      (function walk(nodes, parent, depth) {
        (nodes || []).forEach(function (n) {
          var li = el('li');
          var b = button('kfb-tree-item' + (state.path === n.path ? ' is-active' : ''), n.name);
          b.style.paddingLeft = 8 + depth * 14 + 'px';
          b.title = n.path;
          b.onclick = function () { go(n.path); };
          li.appendChild(b);
          parent.appendChild(li);
          walk(n.children, parent, depth + 1);
        });
      })(state.tree, ul, 1);
      treeCol.appendChild(ul);
    }

    // Listing.
    function visibleEntries() {
      var f = state.filter.toLowerCase();
      return state.entries.filter(function (e) {
        if (f && e.name.toLowerCase().indexOf(f) < 0) return false;
        if (!e.is_dir && mode === 'picker' && accept === 'image' && e.kind !== 'image') return false;
        return true;
      }).sort(function (a, b) {
        if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
        return a.name.localeCompare(b.name);
      });
    }

    function fileURL(name) {
      return state.baseURL.replace(/\/$/, '') + '/' + joinPath(state.path, name);
    }

    function thumb(e) {
      var box = el('div', 'kfb-thumb');
      if (e.is_dir) {
        box.classList.add('kfb-thumb--dir');
      } else if (e.kind === 'image') {
        var img = el('img');
        img.loading = 'lazy';
        img.draggable = false;
        img.alt = '';
        img.src = fileURL(e.name);
        box.appendChild(img);
      } else {
        box.classList.add('kfb-thumb--file');
        box.appendChild(el('span', '', extOf(e.name)));
      }
      return box;
    }

    function renderList() {
      list.innerHTML = '';
      list.className = 'kfb-list kfb-list--' + state.view;
      viewBtn.textContent = state.view === 'grid' ? Kosatka.t('List') : Kosatka.t('Grid');
      var items = visibleEntries();
      if (!items.length) {
        list.appendChild(el('p', 'kfb-empty', state.filter ? Kosatka.t('Nothing found.') : Kosatka.t('No files yet. Upload them with the button or drop them here.')));
        return;
      }
      items.forEach(function (e) {
        var row = el('div', 'kfb-item' + (e.is_dir ? ' kfb-item--dir' : ''));
        var open = button('kfb-item-open', '');
        open.appendChild(thumb(e));
        var name = el('span', 'kfb-item-name', e.name);
        open.appendChild(name);
        open.title = e.name;
        open.onclick = function () {
          if (e.is_dir) go(joinPath(state.path, e.name));
          else if (mode === 'picker') select(e);
          else preview(e);
        };
        row.appendChild(open);

        var meta = el('span', 'kfb-item-meta');
        if (!e.is_dir) {
          var parts = [formatSize(e.size)];
          if (e.width && e.height) parts.push(e.width + '×' + e.height);
          parts.push(formatDate(e.mod_time));
          meta.textContent = parts.filter(Boolean).join(' · ');
        }
        row.appendChild(meta);

        var acts = el('span', 'kfb-item-actions');
        if (!e.is_dir && mode === 'picker') {
          var sel = button('kfb-btn-primary kfb-btn-sm', Kosatka.t('Select'));
          sel.onclick = function () { select(e); };
          acts.appendChild(sel);
        }
        if (!e.is_dir) {
          var copy = button('kfb-btn-sm', Kosatka.t('Copy link'));
          copy.onclick = function () { copyLink(e); };
          acts.appendChild(copy);
        }
        var ren = button('kfb-btn-sm', Kosatka.t('Rename'));
        ren.onclick = function () { rename(e); };
        acts.appendChild(ren);
        var del = button('kfb-btn-sm kfb-btn-danger', Kosatka.t('Delete'));
        del.onclick = function () { remove(e); };
        acts.appendChild(del);
        row.appendChild(acts);
        list.appendChild(row);
      });
    }

    // Actions.
    function select(e) {
      if (opts.onSelect) opts.onSelect(fileURL(e.name), e);
    }

    function copyLink(e) {
      var url = global.location.origin + fileURL(e.name);
      var done = function () { setStatus(Kosatka.t('Link copied: @url', { '@url': url })); };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(url).then(done, function () { Kosatka.prompt(Kosatka.t('Link'), url); });
      } else {
        Kosatka.prompt(Kosatka.t('Link'), url);
      }
    }

    function preview(e) {
      if (e.kind !== 'image') {
        global.open(fileURL(e.name), '_blank', 'noopener');
        return;
      }
      var overlay = el('div', 'kfb-preview');
      var img = el('img');
      img.src = fileURL(e.name);
      img.alt = e.name;
      overlay.appendChild(img);
      overlay.appendChild(el('div', 'kfb-preview-caption', e.name));
      var close = function () {
        overlay.remove();
        document.removeEventListener('keydown', onKey);
      };
      var onKey = function (ev) { if (ev.key === 'Escape') close(); };
      overlay.onclick = close;
      document.addEventListener('keydown', onKey);
      document.body.appendChild(overlay);
    }

    // usageOf - where a file is linked from right now (server reads the
    // texts); folders resolve to no places.
    function usageOf(e) {
      if (e.is_dir) return Promise.resolve([]);
      return request('GET', '/usage', { path: joinPath(state.path, e.name) }).then(function (j) { return j.refs || []; });
    }

    function refsList(refs) {
      var lines = refs.slice(0, 8).map(function (r) { return '- ' + r.title; });
      if (refs.length > 8) lines.push(Kosatka.t('…and @count more', { '@count': refs.length - 8 }));
      return lines.join('\n');
    }

    function rename(e) {
      var current = e.is_dir ? e.name : e.name.replace(/\.[^.]+$/, '');
      usageOf(e).then(function (refs) {
        var question = e.is_dir
          ? Kosatka.t('New folder name (links to files in this folder will stop working):')
          : Kosatka.t('New file name (the extension stays):');
        if (refs.length) {
          question += '\n\n' + Kosatka.t('The file is used in @count places; links there will stop working:', { '@count': refs.length }) + '\n' + refsList(refs);
        }
        return Kosatka.prompt(question, current).then(function (name) {
          if (!name || !name.trim() || name.trim() === current) return;
          return post('/rename', { path: joinPath(state.path, e.name), name: name.trim() })
            .then(function () { setStatus(Kosatka.t('Renamed.')); return refresh(true); });
        });
      }).catch(fail);
    }

    function remove(e) {
      usageOf(e).then(function (refs) {
        var question;
        if (e.is_dir) {
          question = Kosatka.t('Delete folder "@name"? Only an empty folder can be deleted.', { '@name': e.name });
        } else if (refs.length) {
          question = Kosatka.t('"@name" is used in @count places, they will lose it:', { '@name': e.name, '@count': refs.length }) +
            '\n' + refsList(refs) + '\n\n' + Kosatka.t('Delete anyway?');
        } else {
          question = Kosatka.t('Delete file "@name"? It is not used anywhere.', { '@name': e.name });
        }
        return Kosatka.confirm(question).then(function (yes) {
          if (!yes) return;
          return post(e.is_dir ? '/rmdir' : '/unlink', { path: joinPath(state.path, e.name) })
            .then(function () { setStatus(Kosatka.t('Deleted.')); return refresh(e.is_dir); });
        });
      }).catch(fail);
    }

    mkdirBtn.onclick = function () {
      Kosatka.prompt(Kosatka.t('Folder name (Cyrillic is fine - it will be transliterated to Latin on the server):')).then(function (name) {
        if (!name || !name.trim()) return;
        return post('/mkdir', { path: state.path, name: name.trim() })
          .then(function () { return refresh(true); });
      }).catch(fail);
    };

    // Upload: sequential, with progress.
    function uploadOne(file, index, total) {
      return new Promise(function (resolve, reject) {
        var fd = new FormData();
        fd.append('file', file, file.name);
        fd.append('csrf_token', csrf());
        if (state.path) fd.append('path', state.path);
        if (opts.formatId) fd.append('text_format_id', opts.formatId);
        var xhr = new XMLHttpRequest();
        xhr.open('POST', apiURL('/upload'));
        xhr.withCredentials = true;
        xhr.upload.onprogress = function (ev) {
          if (!ev.lengthComputable) return;
          setStatus(Kosatka.t('Uploading @name (@percent%), @current of @total…', {
            '@name': file.name, '@percent': Math.round(ev.loaded * 100 / ev.total), '@current': index, '@total': total,
          }));
        };
        xhr.onload = function () {
          var j = {};
          try { j = JSON.parse(xhr.responseText); } catch (e) { /* not JSON */ }
          if (xhr.status >= 200 && xhr.status < 300) resolve(j);
          else reject(new Error(file.name + ': ' + (j.error || Kosatka.t('Error'))));
        };
        xhr.onerror = function () { reject(new Error(file.name + ': ' + Kosatka.t('Error'))); };
        xhr.send(fd);
      });
    }

    function upload(files) {
      files = Array.prototype.slice.call(files || []);
      if (!files.length) return;
      uploadBtn.disabled = true;
      var errors = [];
      var chain = Promise.resolve();
      files.forEach(function (f, i) {
        chain = chain.then(function () {
          return uploadOne(f, i + 1, files.length).catch(function (err) { errors.push(err.message); });
        });
      });
      chain.then(function () {
        uploadBtn.disabled = false;
        fileInput.value = '';
        var ok = files.length - errors.length;
        if (errors.length) setStatus(Kosatka.t('Uploaded: @ok of @total.', { '@ok': ok, '@total': files.length }) + ' ' + errors.join('; '), true);
        else setStatus(Kosatka.t('Uploaded: @ok of @total.', { '@ok': ok, '@total': files.length }));
        return refresh(false);
      });
    }

    uploadBtn.onclick = function () { fileInput.click(); };
    fileInput.onchange = function () { upload(fileInput.files); };

    // Upload by drag and drop only for files dragged in from outside: a
    // thumbnail dragged within the browser must not upload a copy.
    var dragDepth = 0;
    var innerDrag = false;
    host.addEventListener('dragstart', function () { innerDrag = true; });
    host.addEventListener('dragend', function () { innerDrag = false; });
    main.addEventListener('dragenter', function (ev) {
      if (innerDrag || !ev.dataTransfer || Array.prototype.indexOf.call(ev.dataTransfer.types || [], 'Files') < 0) return;
      ev.preventDefault();
      dragDepth++;
      main.classList.add('is-dragover');
    });
    main.addEventListener('dragover', function (ev) {
      if (main.classList.contains('is-dragover')) ev.preventDefault();
    });
    main.addEventListener('dragleave', function () {
      dragDepth = Math.max(0, dragDepth - 1);
      if (!dragDepth) main.classList.remove('is-dragover');
    });
    main.addEventListener('drop', function (ev) {
      ev.preventDefault();
      var fromOutside = main.classList.contains('is-dragover') && !innerDrag;
      dragDepth = 0;
      innerDrag = false;
      main.classList.remove('is-dragover');
      if (fromOutside && ev.dataTransfer) upload(ev.dataTransfer.files);
    });

    search.addEventListener('input', function () {
      state.filter = search.value.trim();
      renderList();
    });

    viewBtn.onclick = function () {
      state.view = state.view === 'grid' ? 'list' : 'grid';
      try { global.localStorage.setItem('kfbView', state.view); } catch (e) { /* storage unavailable */ }
      renderList();
    };

    // Loading.
    function loadTree() {
      return request('GET', '/tree').then(function (j) { state.tree = j.tree || []; });
    }

    function loadList() {
      return request('GET', '', { path: state.path }).then(function (j) {
        state.entries = j.entries || [];
        state.baseURL = j.base_url || state.baseURL;
        // The site's image formats (upload policy) for the picker filter.
        if (j.image_types && j.image_types.length) imageTypes = j.image_types.join(',');
        if (accept === 'image') fileInput.accept = imageTypes;
        // Without the "upload documents" permission: images only.
        if (j.can_upload_documents === false) {
          fileInput.accept = imageTypes;
          uploadBtn.title = Kosatka.t('Upload images to the current folder (@formats)', { '@formats': j.image_formats || '' });
        }
      });
    }

    function refresh(withTree) {
      return (withTree ? loadTree() : Promise.resolve())
        .then(loadList)
        .then(function () {
          renderCrumbs();
          renderTree();
          renderList();
        })
        .catch(fail);
    }

    function go(path) {
      state.path = path || '';
      state.filter = '';
      search.value = '';
      setStatus('');
      if (opts.onPathChange) opts.onPathChange(state.path);
      return loadList()
        .then(function () {
          renderCrumbs();
          renderTree();
          renderList();
        })
        .catch(function (err) {
          // A remembered folder that no longer exists: back to the root.
          if (state.path) return go('');
          fail(err);
        });
    }

    loadTree().then(function () { return go(state.path); }).catch(fail);

    return {
      reload: function () { return refresh(true); },
      path: function () { return state.path; },
    };
  }

  global.KosatkaFileBrowser = { mount: mount };
})(window);
