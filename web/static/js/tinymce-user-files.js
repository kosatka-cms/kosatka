/**
 * TinyMCE helpers: user image upload (session + CSRF), the file picker
 * (file-browser.js) for /api/user/files, the theme's stylesheets.
 */
(function (global) {
  var NS = 'cmsTinyMCEUserFiles';

  // csrf - the page's current token (csrf_head keeps the meta tag up to
  // date when it fetches one).
  function csrf() {
    var m = document.querySelector('meta[name="csrf-token"]');
    return m ? m.getAttribute('content') : '';
  }

  /**
   * Upload one image (File or Blob) to /api/user/files/upload. subPath = folder under user root.
   * formatId = active text format's ID (per-format upload policy), omitted for fixed-format editors.
   */
  function uploadImageBlob(blob, filename, subPath, formatId) {
    var fd = new FormData();
    fd.append('file', blob, filename || 'image');
    fd.append('csrf_token', csrf());
    if (subPath) {
      fd.append('path', subPath);
    }
    if (formatId) {
      fd.append('text_format_id', formatId);
    }
    return fetch('/api/user/files/upload', {
      method: 'POST',
      body: fd,
      credentials: 'same-origin',
    })
      .then(function (res) {
        return res.json().then(function (j) {
          return { res: res, j: j };
        });
      })
      .then(function (x) {
        if (!x.res.ok) {
          throw new Error((x.j && x.j.error) || Kosatka.t('Upload failed'));
        }
        if (!x.j.location) {
          throw new Error(Kosatka.t('The server did not return the file address'));
        }
        return x.j.location;
      });
  }

  function syncUploadPath(path) {
    global[NS] = global[NS] || {};
    global[NS].uploadPath = path || '';
  }

  // The file browser (file-browser.js/css) is loaded on first use, from
  // the same static version as this script.
  var scriptSrc = (document.currentScript && document.currentScript.src) || '';
  // The theme's editor stylesheets (theme_editor_css, on this script's
  // tag): the text being edited looks as on the page.
  var themeCSS = (document.currentScript && document.currentScript.getAttribute('data-editor-css')) || '';
  var browserLoading = null;

  function assetURL(dir, name, ext) {
    var m = scriptSrc.match(/\?v=[^&#]*/);
    var min = /\.min\.js/.test(scriptSrc) ? '.min' : '';
    return '/static/' + dir + '/' + name + min + '.' + ext + (m ? m[0] : '');
  }

  function loadFileBrowser() {
    if (global.KosatkaFileBrowser) return Promise.resolve();
    if (browserLoading) return browserLoading;
    browserLoading = new Promise(function (resolve, reject) {
      var css = document.createElement('link');
      css.rel = 'stylesheet';
      css.href = assetURL('css', 'file-browser', 'css');
      document.head.appendChild(css);
      var js = document.createElement('script');
      js.src = assetURL('js', 'file-browser', 'js');
      js.onload = function () { resolve(); };
      js.onerror = function () {
        browserLoading = null;
        reject(new Error('file browser failed to load'));
      };
      document.head.appendChild(js);
    });
    return browserLoading;
  }

  // openPicker shows the user's files in a window (Kosatka.window); accept
  // 'image' for the image dialog, 'any' for links. callback gets the URL
  // and TinyMCE meta.
  function openPicker(callback, formatId, accept) {
    global[NS] = global[NS] || {};
    loadFileBrowser().then(function () {
      var win = Kosatka.window({
        title: accept === 'image' ? Kosatka.t('Your images') : Kosatka.t('Your files'),
        className: 'kfb-window',
      });

      global.KosatkaFileBrowser.mount(win.body, {
        mode: 'picker',
        accept: accept,
        formatId: formatId,
        path: global[NS].uploadPath || '',
        csrfToken: csrf(),
        onPathChange: syncUploadPath,
        onSelect: function (url, entry) {
          var meta = entry.kind === 'image' ? { alt: '' } : { text: entry.name, title: entry.name };
          if (entry.width) meta.width = String(entry.width);
          if (entry.height) meta.height = String(entry.height);
          callback(url, meta);
          win.close();
        },
      });
    }).catch(function (err) {
      Kosatka.alert(err.message || String(err));
    });
  }

  function toolbarHasImage(cfg) {
    var t = cfg.toolbar;
    if (typeof t !== 'string') {
      return false;
    }
    var parts = t.split(/\s+/);
    for (var i = 0; i < parts.length; i++) {
      if (parts[i] === 'image') {
        return true;
      }
    }
    return false;
  }

  function mergeConfig(cfg, formatId) {
    cfg = cfg || {};
    if (!cfg.license_key) cfg.license_key = 'gpl';
    cfg.image_advtab = true;
    // Buttons that don't fit go to a second row, not behind a "…" button
    // (a format's long toolbar, the modules' buttons after it).
    if (!cfg.toolbar_mode) cfg.toolbar_mode = 'wrap';
    // Links and images as typed. By default TinyMCE rewrites this site's
    // URLs relative to the editing page ("/articles/x" saved from
    // /node/5/edit becomes "../../articles/x"), which breaks them on any
    // other page. Root-relative paths stay so; an absolute URL to this
    // site loses its host. The base is the site root, not the editing
    // page: a relative link already in old content ("../articles/x")
    // resolves against it and is saved as "/articles/x".
    if (cfg.relative_urls === undefined) cfg.relative_urls = false;
    if (cfg.remove_script_host === undefined) cfg.remove_script_host = true;
    if (!cfg.document_base_url) cfg.document_base_url = global.location.origin + '/';
    if (cfg.content_css === undefined && themeCSS) {
      cfg.content_css = themeCSS;
      if (cfg.content_style === undefined) cfg.content_style = 'body { background: #fff; }';
    }

    cfg.images_upload_handler = function (blobInfo, progress) {
      return new Promise(function (resolve, reject) {
        var up =
          global[NS] && global[NS].uploadPath !== undefined && global[NS].uploadPath !== null
            ? global[NS].uploadPath
            : '';
        uploadImageBlob(blobInfo.blob(), blobInfo.filename(), up, formatId)
          .then(resolve)
          .catch(function (e) {
            reject(e.message || String(e));
          });
      });
    };

    cfg.file_picker_callback = function (cb, value, meta) {
      if (meta.filetype === 'image') openPicker(cb, formatId, 'image');
      else if (meta.filetype === 'file') openPicker(cb, formatId, 'any');
    };
    cfg.file_picker_types = 'file image';

    // Same button as the Insert → Image menu item (image plugin, mceImage command).
    if (typeof cfg.toolbar === 'string' && !toolbarHasImage(cfg)) {
      cfg.toolbar = cfg.toolbar.replace(/\s+$/, '') + ' | image';
    }

    return cfg;
  }

  global[NS] = global[NS] || {};
  global[NS].mergeConfig = mergeConfig;
})(window);
