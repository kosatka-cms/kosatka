// Video embed module (entity.Module.JS): a click on a video's preview in a
// text (templates/video_embed/embed.html, .video-embed-play) puts its
// player there, starting at once; without the script the preview is a
// link to the video.
(function initVideoEmbeds() {
    document.addEventListener('click', function(e) {
        const link = e.target.closest('.video-embed-play[data-video-player]');
        if (!link || e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey) {
            return;
        }
        e.preventDefault();
        const player = document.createElement('iframe');
        player.className = 'video-embed-player';
        player.src = link.dataset.videoPlayer;
        player.title = link.dataset.videoTitle || '';
        player.allow = 'autoplay; encrypted-media; fullscreen; picture-in-picture';
        player.allowFullscreen = true;
        player.referrerPolicy = 'strict-origin-when-cross-origin';
        link.replaceWith(player);
        player.focus();
    });
})();
