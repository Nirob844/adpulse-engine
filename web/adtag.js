/**
 * Mini Ad Server - Client-Side Ad Tag Library
 * 
 * Features:
 * - Scans DOM for ad slots: <div class="ad-slot" data-ad-size="300x250"></div>
 * - Asynchronously fetches ads from GET /ad?size=WxH
 * - Wraps creative in tracking click_url (navigating through /track/click?c=...)
 * - Listens for creative image load event and fires impression pixel (/track/impression?c=...)
 * - Enforces strict once-per-load impression tracking
 * - Gracefully handles HTTP 204 No Content (empty inventory)
 * - Emits custom DOM events for publisher diagnostics and live event logging
 */

(function (window, document) {
    'use strict';

    var MiniAdTag = {
        version: '1.0.0',
        apiBase: (typeof window !== 'undefined' && window.location && window.location.port !== '8000') ? 'http://localhost:8000' : '',

        /**
         * Initialize all ad slots on the page.
         */
        init: function (options) {
            options = options || {};
            if (options.apiBase) {
                this.apiBase = options.apiBase.replace(/\/$/, '');
            }

            var slots = document.querySelectorAll('.ad-slot[data-ad-size]');
            for (var i = 0; i < slots.length; i++) {
                this.loadSlot(slots[i]);
            }
        },

        /**
         * Load or refresh a specific ad slot element.
         * @param {HTMLElement|string} slot - The DOM element or element ID
         */
        loadSlot: function (slot) {
            if (typeof slot === 'string') {
                slot = document.getElementById(slot);
            }
            if (!slot) return;

            var size = slot.getAttribute('data-ad-size');
            if (!size) {
                console.warn('[MiniAdTag] Missing data-ad-size on slot:', slot);
                return;
            }

            var parts = size.toLowerCase().split('x');
            var width = parseInt(parts[0], 10);
            var height = parseInt(parts[1], 10);

            // Set slot dimensions and styling
            slot.style.width = width ? width + 'px' : 'auto';
            slot.style.minHeight = height ? height + 'px' : 'auto';
            slot.classList.add('ad-slot-loading');
            slot.innerHTML = '<div class="ad-slot-spinner" title="Loading ad..."></div>';

            var adUrl = (this.apiBase || '') + '/ad?size=' + encodeURIComponent(size);
            var self = this;

            fetch(adUrl, {
                method: 'GET',
                headers: {
                    'Accept': 'application/json'
                }
            })
            .then(function (response) {
                if (response.status === 204) {
                    self._handleNoContent(slot, size);
                    return null;
                }
                if (!response.ok) {
                    throw new Error('Ad server returned status ' + response.status);
                }
                return response.json();
            })
            .then(function (adData) {
                if (!adData) return;
                self._renderAd(slot, size, adData);
            })
            .catch(function (error) {
                console.error('[MiniAdTag] Error loading ad for slot ' + size + ':', error);
                self._handleError(slot, size, error);
            });
        },

        /**
         * Refresh all ad slots on the page.
         */
        refreshAll: function () {
            var slots = document.querySelectorAll('.ad-slot[data-ad-size]');
            for (var i = 0; i < slots.length; i++) {
                this.loadSlot(slots[i]);
            }
        },

        /**
         * Render creative into slot and wire tracking handlers.
         */
        _renderAd: function (slot, size, adData) {
            slot.classList.remove('ad-slot-loading');
            slot.innerHTML = '';

            var creativeId = adData.creative_id;
            var clickUrl = adData.click_url;
            var impressionUrl = adData.impression_url;

            // Ensure absolute tracking URLs when loaded across ports
            if (this.apiBase) {
                if (clickUrl && clickUrl.indexOf('http') !== 0) {
                    clickUrl = this.apiBase + clickUrl;
                }
                if (impressionUrl && impressionUrl.indexOf('http') !== 0) {
                    impressionUrl = this.apiBase + impressionUrl;
                }
            }

            // 1. Create tracking click anchor
            var anchor = document.createElement('a');
            anchor.href = clickUrl;
            anchor.target = '_blank';
            anchor.rel = 'noopener noreferrer';
            anchor.className = 'ad-link';
            anchor.setAttribute('data-creative-id', creativeId);
            anchor.setAttribute('aria-label', 'Advertisement creative #' + creativeId);

            // 2. Insert creative HTML (or construct <img> fallback)
            if (adData.html) {
                anchor.innerHTML = adData.html;
            } else if (adData.image_url) {
                var imgUrl = adData.image_url;
                if (this.apiBase && imgUrl.indexOf('http') !== 0) {
                    imgUrl = this.apiBase + imgUrl;
                }
                var parts = size.toLowerCase().split('x');
                var img = document.createElement('img');
                img.src = imgUrl;
                img.width = parseInt(parts[0], 10) || 300;
                img.height = parseInt(parts[1], 10) || 250;
                img.alt = 'Advertisement';
                anchor.appendChild(img);
            }

            slot.appendChild(anchor);

            // 3. Impression Tracking - Fire ONLY when creative image loads in browser, and at most once
            var imgElem = anchor.querySelector('img');
            var impressionFired = false;
            var self = this;

            function triggerImpression() {
                if (impressionFired) return;
                impressionFired = true;

                // Fire impression tracking beacon
                var beacon = new Image();
                beacon.src = impressionUrl;

                var eventDetail = {
                    slotId: slot.id || null,
                    size: size,
                    creativeId: creativeId,
                    impressionUrl: impressionUrl,
                    clickUrl: clickUrl,
                    timestamp: new Date().toISOString()
                };

                // Dispatch custom event for real-time diagnostics
                var ev = new CustomEvent('miniad:impression', { detail: eventDetail });
                window.dispatchEvent(ev);
                console.log('[MiniAdTag] Impression tracked for creative #' + creativeId + ' on slot ' + size);
            }

            if (imgElem) {
                // If relative src and apiBase is set, fix src
                if (self.apiBase && imgElem.getAttribute('src') && imgElem.getAttribute('src').indexOf('http') !== 0) {
                    imgElem.src = self.apiBase + imgElem.getAttribute('src');
                }

                // Error fallback: If external image fails (e.g. placehold.co blocked by ISP or ad blocker), fallback to reliable local SVG banner
                imgElem.addEventListener('error', function () {
                    var fallbackSvg = (self.apiBase || '') + '/assets/banner_' + size + '.svg';
                    if (imgElem.src !== fallbackSvg) {
                        console.warn('[MiniAdTag] Creative image #' + creativeId + ' failed to load. Switching to local SVG banner:', fallbackSvg);
                        imgElem.src = fallbackSvg;
                    }
                }, { once: true });

                if (imgElem.complete && imgElem.naturalWidth > 0) {
                    // Image was already cached and loaded immediately
                    triggerImpression();
                } else {
                    imgElem.addEventListener('load', triggerImpression, { once: true });
                }
            } else {
                // If creative has no image tag, trigger immediately
                triggerImpression();
            }

            // Click listener for diagnostic event emission
            anchor.addEventListener('click', function () {
                var ev = new CustomEvent('miniad:click', {
                    detail: {
                        slotId: slot.id || null,
                        size: size,
                        creativeId: creativeId,
                        clickUrl: clickUrl,
                        timestamp: new Date().toISOString()
                    }
                });
                window.dispatchEvent(ev);
            });

            // Dispatch ad loaded event
            var loadEvent = new CustomEvent('miniad:loaded', {
                detail: {
                    slotId: slot.id || null,
                    size: size,
                    creativeId: creativeId,
                    clickUrl: clickUrl,
                    impressionUrl: impressionUrl,
                    timestamp: new Date().toISOString()
                }
            });
            window.dispatchEvent(loadEvent);
        },

        /**
         * Handle HTTP 204 No Content (no active creative or frequency cap reached)
         */
        _handleNoContent: function (slot, size) {
            slot.classList.remove('ad-slot-loading');
            slot.classList.add('ad-slot-empty');
            slot.innerHTML = '<div class="ad-empty-notice">' +
                '<span class="ad-empty-badge">204 No Content</span>' +
                '<span class="ad-empty-text">No active ad available for ' + size + '</span>' +
                '</div>';

            var emptyEvent = new CustomEvent('miniad:empty', {
                detail: {
                    slotId: slot.id || null,
                    size: size,
                    status: 204,
                    timestamp: new Date().toISOString()
                }
            });
            window.dispatchEvent(emptyEvent);
            console.log('[MiniAdTag] No ad content available for slot ' + size + ' (HTTP 204)');
        },

        /**
         * Handle fetch error
         */
        _handleError: function (slot, size, error) {
            slot.classList.remove('ad-slot-loading');
            slot.classList.add('ad-slot-error');
            slot.innerHTML = '<div class="ad-empty-notice error">' +
                '<span class="ad-empty-badge">Error</span>' +
                '<span class="ad-empty-text">Failed to load ' + size + '</span>' +
                '</div>';

            var errEvent = new CustomEvent('miniad:error', {
                detail: {
                    slotId: slot.id || null,
                    size: size,
                    error: error.message,
                    timestamp: new Date().toISOString()
                }
            });
            window.dispatchEvent(errEvent);
        }
    };

    // Export globally
    window.MiniAdTag = MiniAdTag;

    // Auto-initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function () {
            MiniAdTag.init();
        });
    } else {
        MiniAdTag.init();
    }

})(window, document);
