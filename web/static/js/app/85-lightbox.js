// ===== PhotoSwipe lightbox =====

let contentGalleryLightbox = null;

// A content image link marked data-lightbox-skip (e.g. a module's "view
// the whole gallery") is followed, not opened in the lightbox - bound once
// per document.
document.addEventListener('click', function(e) {
    const target = e.target.closest('a[data-lightbox-skip]');
    if (target) {
        e.stopPropagation();
        window.location.href = target.getAttribute('href');
    }
}, true);

// contentGallery - images in content open in PhotoSwipe. The lightbox
// binds the galleries present when it starts, so HTML with a gallery put
// in the page later restarts it.
var contentGallerySelector = '.node-content, .item-content, [data-lightbox]';
Kosatka.behaviors.contentGallery = {
    attach: function (context) {
        if (context === document || context.matches(contentGallerySelector) || context.querySelector(contentGallerySelector)) {
            initContentGallery();
        }
    }
};

function initContentGallery() {
    if (typeof PhotoSwipeLightbox === 'undefined') {
        console.warn('PhotoSwipe is not loaded');
        return;
    }

    if (contentGalleryLightbox) {
        try {
            contentGalleryLightbox.destroy();
        } catch (err) {
            console.warn('PhotoSwipe destroy', err);
        }
        contentGalleryLightbox = null;
    }

    const lightbox = new PhotoSwipeLightbox({
        // [data-lightbox] - any other container of image links (a module's,
        // a theme's own page layout).
        gallery: contentGallerySelector,
        children: 'a.content-image-link:not([data-lightbox-skip])',
        pswpModule: PhotoSwipe,
        bgOpacity: 0.9,
        padding: { top: 20, bottom: 40, left: 20, right: 20 },
        loop: false,
        pinchToClose: true,
        closeOnVerticalDrag: true,
        escKey: true,
        arrowKeys: true,
        // Tapping/clicking the image closes the lightbox (handy on phones; double-tap zoom is off)
        tapAction: 'close',
        imageClickAction: 'close',
        doubleTapAction: false,
    });

    lightbox.on('uiRegister', function() {
        lightbox.pswp.ui.registerElement({
            name: 'custom-caption',
            order: 9,
            isButton: false,
            appendTo: 'root',
            html: '',
            onInit: (el, pswp) => {
                const applyCaption = () => {
                    const currSlideElement = pswp.currSlide && pswp.currSlide.data ? pswp.currSlide.data.element : null;
                    let captionHTML = '';
                    if (currSlideElement) {
                        captionHTML = currSlideElement.getAttribute('data-pswp-caption-html') || '';
                    }
                    el.innerHTML = captionHTML;
                };
                pswp.on('change', applyCaption);
                pswp.on('openingAnimationEnd', applyCaption);
            }
        });
    });

    lightbox.init();
    contentGalleryLightbox = lightbox;
}

