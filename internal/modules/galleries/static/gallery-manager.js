// gallery-manager.js - the gallery editor (edit_gallery.html,
// [data-gallery-editor] with the gallery's and data-node-id the content's
// ID): its settings, uploads, the images' order, the cover and the
// images' titles; and the content's galleries (galleries_list.html): a
// new one created (#createGalleryForm, data-node-id).

document.addEventListener('DOMContentLoaded', function() {
    const editor = document.querySelector('[data-gallery-editor]');
    if (editor) {
        initGalleryEditor(parseInt(editor.dataset.galleryEditor, 10), parseInt(editor.dataset.nodeId, 10));
    }
    const createForm = document.getElementById('createGalleryForm');
    const saveGalleryBtn = document.getElementById('saveGalleryBtn');
    if (createForm && saveGalleryBtn) {
        saveGalleryBtn.addEventListener('click', function() { createGallery(createForm); });
    }
});

// createGallery - the gallery the form describes, for its content; the
// page reloads with it.
async function createGallery(form) {
    const params = new URLSearchParams();
    for (const [key, value] of new FormData(form).entries()) {
        params.append(key, value);
    }
    try {
        const response = await fetch(`/api/node/${form.dataset.nodeId}/galleries`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded',
            },
            body: params.toString()
        });
        if (response.ok) {
            window.location.reload();
        } else {
            const data = await response.json();
            Kosatka.alert(Kosatka.t('Error: @message', {'@message': data.error || Kosatka.t('Could not create the gallery')}));
        }
    } catch (error) {
        console.error('Error creating gallery:', error);
        Kosatka.alert(Kosatka.t('Error creating the gallery'));
    }
}

let galleryBulkUploadInProgress = false;

function initGalleryEditor(galleryId, nodeId) {
    const saveSettingsBtn = document.getElementById('saveSettingsBtn');
    if (saveSettingsBtn) {
        saveSettingsBtn.addEventListener('click', async () => {
            await saveGallerySettings(galleryId);
        });
    }

    const uploadForm = document.getElementById('uploadForm');
    if (uploadForm) {
        uploadForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            if (galleryBulkUploadInProgress) {
                return;
            }
            await uploadImages(galleryId);
        });
    }

    const imagesGrid = document.getElementById('imagesGrid');
    if (imagesGrid && imagesGrid.children.length > 0) {
        initSortable(imagesGrid, galleryId);
    }

    document.querySelectorAll('.set-cover-btn').forEach(btn => {
        btn.addEventListener('click', async function() {
            const itemId = parseInt(this.dataset.itemId);
            await setCover(galleryId, itemId);
        });
    });

    document.querySelectorAll('.set-node-main-btn').forEach(btn => {
        btn.addEventListener('click', async function() {
            const itemId = parseInt(this.dataset.itemId);
            await setNodeMainImage(galleryId, itemId);
        });
    });

    document.querySelectorAll('.edit-meta-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            openEditMetaModal(this.dataset.itemId, this.dataset.title, this.dataset.description);
        });
    });

    document.querySelectorAll('.delete-item-btn').forEach(btn => {
        btn.addEventListener('click', async function() {
            const itemId = parseInt(this.dataset.itemId);
            if (await Kosatka.confirm(Kosatka.t('Are you sure you want to delete this image?'))) {
                await deleteItem(itemId);
            }
        });
    });

    const saveMetaBtn = document.getElementById('saveMetaBtn');
    if (saveMetaBtn) {
        saveMetaBtn.addEventListener('click', async () => {
            await saveItemMetadata();
        });
    }
}

async function saveGallerySettings(galleryId) {
    const form = document.getElementById('gallerySettingsForm');
    const formData = new FormData(form);

    const params = new URLSearchParams();
    for (const [key, value] of formData.entries()) {
        params.append(key, value);
    }

    try {
        const response = await fetch(`/api/gallery/${galleryId}`, {
            method: 'PATCH',
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded',
            },
            body: params.toString()
        });

        if (response.ok) {
            const data = await response.json();
            Kosatka.toast(Kosatka.t('Gallery settings saved'), 'success');
            if (data.gallery) {
                const titleInput = document.getElementById('galleryTitle');
                const descInput = document.getElementById('galleryDescription');

                // Title and Description may be null (unset).
                if (titleInput) {
                    titleInput.value = (data.gallery.Title != null) ? data.gallery.Title : '';
                }
                if (descInput) {
                    descInput.value = (data.gallery.Description != null) ? data.gallery.Description : '';
                }

                const pageTitle = document.querySelector('.node-title');
                if (pageTitle) {
                    pageTitle.textContent = data.gallery.Title || Kosatka.t('Gallery');
                }
            }
        } else {
            const data = await response.json();
            Kosatka.toast(Kosatka.t('Error: @error', {'@error': data.error || Kosatka.t('Could not save the settings')}), 'error');
        }
    } catch (error) {
        console.error('Error saving gallery settings:', error);
        Kosatka.toast(Kosatka.t('Error saving the settings'), 'error');
    }
}

async function parseJSONErrorResponse(response) {
    try {
        const data = await response.json();
        if (data && data.error) {
            return String(data.error);
        }
    } catch (_) {
        // ignore invalid JSON
    }
    if (response.status === 413) {
        return Kosatka.t('The file is too large for the server');
    }
    return response.statusText || Kosatka.t('HTTP error @status', {'@status': response.status});
}

function clearGalleryUploadErrors() {
    const el = document.getElementById('galleryUploadErrors');
    if (!el) {
        return;
    }
    el.style.display = 'none';
    el.textContent = '';
}

function showGalleryUploadErrors(lines) {
    const el = document.getElementById('galleryUploadErrors');
    if (!el || !lines.length) {
        return;
    }
    el.textContent = lines.join('\n');
    el.style.display = 'block';
    el.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
}

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

// Uploads one file, retrying on 429 (CDN/edge rate limiting) with backoff
// honoring Retry-After when present. Returns the fetch Response (ok or last failed).
async function uploadGalleryFileWithRetry(galleryId, file, maxRetries) {
    let attempt = 0;
    for (;;) {
        const formData = new FormData();
        formData.append('image', file);
        const response = await fetch(`/api/gallery/${galleryId}/items`, {
            method: 'POST',
            body: formData
        });

        if (response.status !== 429 || attempt >= maxRetries) {
            return response;
        }

        const retryAfterHeader = parseInt(response.headers.get('Retry-After'), 10);
        const delayMs = !isNaN(retryAfterHeader)
            ? retryAfterHeader * 1000
            : 1000 * Math.pow(2, attempt); // 1s, 2s, 4s, ...
        attempt++;
        await sleep(delayMs);
    }
}

async function uploadImages(galleryId) {
    const fileInput = document.getElementById('imageFiles');
    const uploadBtn = document.getElementById('uploadBtn');
    const files = fileInput.files;

    if (files.length === 0) {
        Kosatka.toast(Kosatka.t('Select files to upload'), 'warning');
        return;
    }

    if (galleryBulkUploadInProgress) {
        return;
    }
    galleryBulkUploadInProgress = true;
    if (uploadBtn) {
        uploadBtn.disabled = true;
    }
    if (fileInput) {
        fileInput.disabled = true;
    }

    const uploadProgress = document.getElementById('uploadProgress');
    const progressBar = uploadProgress.querySelector('.progress-bar');
    uploadProgress.style.display = 'block';
    clearGalleryUploadErrors();

    let uploaded = 0;
    const total = files.length;
    const uploadErrors = [];

    try {
        for (let i = 0; i < files.length; i++) {
            const file = files[i];

            // Small gap between uploads so bulk uploads (dozens of files) don't
            // trip CDN/edge rate limiting; uploadGalleryFileWithRetry
            // additionally retries with backoff if a 429 slips through anyway.
            if (i > 0) {
                await sleep(400);
            }

            try {
                const response = await uploadGalleryFileWithRetry(galleryId, file, 5);

                if (response.ok) {
                    uploaded++;
                    const percent = Math.round((uploaded / total) * 100);
                    progressBar.style.width = percent + '%';
                    progressBar.textContent = percent + '%';
                    progressBar.setAttribute('aria-valuenow', percent);
                } else {
                    const errMsg = await parseJSONErrorResponse(response);
                    const line = `${file.name}: ${errMsg}`;
                    uploadErrors.push(line);
                    console.error('Error uploading file:', file.name, errMsg);
                    Kosatka.toast(line, 'error', 8000);
                }
            } catch (error) {
                const line = `${file.name}: ${error.message || Kosatka.t('network error')}`;
                uploadErrors.push(line);
                console.error('Error uploading file:', file.name, error);
                Kosatka.toast(line, 'error', 8000);
            }
        }
    } finally {
        galleryBulkUploadInProgress = false;
        if (uploadBtn) {
            uploadBtn.disabled = false;
        }
        if (fileInput) {
            fileInput.disabled = false;
        }
    }

    if (uploadErrors.length > 0) {
        showGalleryUploadErrors(uploadErrors);
    }

    if (uploaded === total) {
        Kosatka.toast(Kosatka.formatPlural(uploaded, 'Uploaded @count image', 'Uploaded @count images'), 'success');
        setTimeout(() => {
            window.location.reload();
        }, 1000);
    } else if (uploaded > 0) {
        Kosatka.toast(Kosatka.t('Uploaded @uploaded of @total. Not uploaded: @failed', {'@uploaded': uploaded, '@total': total, '@failed': uploadErrors.length}), 'warning', 8000);
        if (uploadErrors.length === 0) {
            setTimeout(() => window.location.reload(), 2000);
        }
    } else {
        Kosatka.toast(
            uploadErrors.length
                ? Kosatka.t('Could not upload the files (@count). See the error list below.', {'@count': uploadErrors.length})
                : Kosatka.t('Could not upload the files'),
            'error',
            8000
        );
    }
}

// initSortable - the images' new order by dragging (Kosatka.sortable,
// left or right of an image in the grid), saved once dropped.
function initSortable(element, galleryId) {
    Kosatka.sortable(element, {
        items: '.image-item',
        axis: 'x',
        end: function() { saveOrder(galleryId); }
    });
}

async function saveOrder(galleryId) {
    const items = document.querySelectorAll('#imagesGrid .image-item');
    const itemsOrder = [];

    items.forEach((item, index) => {
        itemsOrder.push({
            id: parseInt(item.dataset.itemId),
            weight: index
        });
    });

    try {
        const response = await fetch(`/api/gallery/${galleryId}/reorder`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ items: itemsOrder })
        });

        if (response.ok) {
            Kosatka.toast(Kosatka.t('Image order saved'), 'success');
        } else {
            const data = await response.json();
            Kosatka.toast(Kosatka.t('Error: @error', {'@error': data.error || Kosatka.t('Could not save the order')}), 'error');
        }
    } catch (error) {
        console.error('Error saving order:', error);
        Kosatka.toast(Kosatka.t('Error saving the order'), 'error');
    }
}

async function setCover(galleryId, itemId) {
    const params = new URLSearchParams();
    params.append('item_id', itemId);

    try {
        const response = await fetch(`/api/gallery/${galleryId}/set-cover`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded',
            },
            body: params.toString()
        });

        if (response.ok) {
            Kosatka.toast(Kosatka.t('Cover set'), 'success');
            document.querySelectorAll('.set-cover-btn').forEach(btn => {
                btn.textContent = '☆';
                btn.closest('.image-item').querySelector('.cover-badge')?.remove();
            });
            const coverBtn = document.querySelector(`.set-cover-btn[data-item-id="${itemId}"]`);
            coverBtn.textContent = '★';
            const coverBadge = document.createElement('span');
            coverBadge.className = 'cover-badge';
            coverBadge.textContent = Kosatka.t('Cover');
            coverBtn.closest('.image-item').querySelector('.image-preview').appendChild(coverBadge);
        } else {
            const data = await response.json();
            Kosatka.toast(Kosatka.t('Error: @error', {'@error': data.error || Kosatka.t('Could not set the cover')}), 'error');
        }
    } catch (error) {
        console.error('Error setting cover:', error);
        Kosatka.toast(Kosatka.t('Error setting the cover'), 'error');
    }
}

async function setNodeMainImage(galleryId, itemId) {
    const confirmed = await Kosatka.confirm(Kosatka.t('Set this image as the main content image?'));
    if (!confirmed) return;

    const params = new URLSearchParams();
    params.append('item_id', itemId);

    try {
        const response = await fetch(`/api/gallery/${galleryId}/set-node-main-image`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded',
            },
            body: params.toString()
        });

        if (response.ok) {
            Kosatka.toast(Kosatka.t('Main content image set'), 'success');
        } else {
            const data = await response.json();
            Kosatka.toast(Kosatka.t('Error: @error', {'@error': data.error || Kosatka.t('Could not set the main image')}), 'error');
        }
    } catch (error) {
        console.error('Error setting node main image:', error);
        Kosatka.toast(Kosatka.t('Error setting the main image'), 'error');
    }
}

function openEditMetaModal(itemId, title, description) {
    document.getElementById('editItemId').value = itemId;
    document.getElementById('editItemTitle').value = title || '';
    document.getElementById('editItemDescription').value = description || '';

    const modal = new bootstrap.Modal(document.getElementById('editMetaModal'));
    modal.show();
}

async function saveItemMetadata() {
    const itemId = document.getElementById('editItemId').value;
    const form = document.getElementById('editMetaForm');
    const formData = new FormData(form);

    const params = new URLSearchParams();
    for (const [key, value] of formData.entries()) {
        params.append(key, value);
    }

    try {
        const response = await fetch(`/api/gallery/items/${itemId}`, {
            method: 'PATCH',
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded',
            },
            body: params.toString()
        });

        if (response.ok) {
            Kosatka.toast(Kosatka.t('Metadata saved'), 'success');

            const title = document.getElementById('editItemTitle').value;
            const description = document.getElementById('editItemDescription').value;

            const editBtn = document.querySelector(`.edit-meta-btn[data-item-id="${itemId}"]`);
            if (editBtn) {
                editBtn.setAttribute('data-title', title);
                editBtn.setAttribute('data-description', description);
            }

            const itemElement = document.querySelector(`.image-item[data-item-id="${itemId}"]`);
            if (itemElement) {
                let titleDiv = itemElement.querySelector('.image-title');
                if (title) {
                    if (!titleDiv) {
                        titleDiv = document.createElement('div');
                        titleDiv.className = 'image-title';
                        itemElement.appendChild(titleDiv);
                    }
                    titleDiv.textContent = title;
                } else {
                    titleDiv?.remove();
                }
            }

            const modal = bootstrap.Modal.getInstance(document.getElementById('editMetaModal'));
            modal.hide();
        } else {
            const data = await response.json();
            Kosatka.toast(Kosatka.t('Error: @error', {'@error': data.error || Kosatka.t('Could not save the metadata')}), 'error');
        }
    } catch (error) {
        console.error('Error saving metadata:', error);
        Kosatka.toast(Kosatka.t('Error saving the metadata'), 'error');
    }
}

async function deleteItem(itemId) {
    try {
        const response = await fetch(`/api/gallery/items/${itemId}`, {
            method: 'DELETE'
        });

        if (response.ok) {
            Kosatka.toast(Kosatka.t('Image deleted'), 'success');
            const itemElement = document.querySelector(`.image-item[data-item-id="${itemId}"]`);
            if (itemElement) {
                itemElement.remove();
            }
        } else {
            const data = await response.json();
            Kosatka.toast(Kosatka.t('Error: @error', {'@error': data.error || Kosatka.t('Could not delete the image')}), 'error');
        }
    } catch (error) {
        console.error('Error deleting item:', error);
        Kosatka.toast(Kosatka.t('Error deleting the image'), 'error');
    }
}
