/**
 * AdPulse Engine - Client-Side Ad Tag Library
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

    var AdPulseTag = {
        version: '1.0.0',
        apiBase: (function () {
            if (typeof window !== 'undefined' && window.ADSERVER_URL) {
                return window.ADSERVER_URL.replace(/\/$/, '');
            }
            if (typeof document !== 'undefined' && document.currentScript && document.currentScript.src) {
                try {
                    var scriptUrl = new URL(document.currentScript.src);
                    // If loaded locally on dev frontend server (e.g. Live Server on port 5500), route to backend on 8000
                    if ((scriptUrl.hostname === 'localhost' || scriptUrl.hostname === '127.0.0.1') && scriptUrl.port && scriptUrl.port !== '8000') {
                        return 'http://localhost:8000';
                    }
                    if (scriptUrl.origin && scriptUrl.origin !== 'null') return scriptUrl.origin;
                } catch (e) {}
            }
            if (typeof window !== 'undefined' && window.location) {
                var host = window.location.hostname;
                var port = window.location.port;
                if ((host === 'localhost' || host === '127.0.0.1') && port && port !== '8000') {
                    return 'http://localhost:8000';
                }
                return window.location.origin || '';
            }
            return '';
        })(),
        simulatedIP: '',

        /**
         * Set simulated client IP for testing frequency capping and refresh slots.
         * @param {string} ip
         */
        setSimulatedIP: function (ip) {
            this.simulatedIP = ip ? ip.trim() : '';
            this.refreshAll();
        },

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
                console.warn('[AdPulseTag] Missing data-ad-size on slot:', slot);
                return;
            }

            var parts = size.toLowerCase().split('x');
            var width = parseInt(parts[0], 10);
            var height = parseInt(parts[1], 10);

            // Set slot dimensions and styling
            if (width && height) {
                slot.style.width = width + 'px';
                slot.style.height = height + 'px';
            }
            slot.classList.add('ad-slot-loading');
            slot.innerHTML = '<div class="ad-slot-spinner" title="Loading ad..."></div>';

            var adUrl = (this.apiBase || '') + '/ad?size=' + encodeURIComponent(size);
            if (this.simulatedIP) {
                adUrl += '&ip=' + encodeURIComponent(this.simulatedIP);
            }
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
                console.error('[AdPulseTag] Error loading ad for slot ' + size + ':', error);
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

            // Propagate simulated client IP to tracking beacons
            if (this.simulatedIP) {
                var ipQuery = 'ip=' + encodeURIComponent(this.simulatedIP);
                if (clickUrl.indexOf('ip=') === -1) {
                    clickUrl += (clickUrl.indexOf('?') === -1 ? '?' : '&') + ipQuery;
                }
                if (impressionUrl.indexOf('ip=') === -1) {
                    impressionUrl += (impressionUrl.indexOf('?') === -1 ? '?' : '&') + ipQuery;
                }
            }

            // 1. Create tracking click anchor
            var anchor = document.createElement('a');
            anchor.href = clickUrl;
            anchor.target = '_blank';
            anchor.rel = 'noopener noreferrer';
            anchor.className = 'ad-link';
            anchor.style.display = 'flex';
            anchor.style.alignItems = 'center';
            anchor.style.justifyContent = 'center';
            anchor.style.width = '100%';
            anchor.style.height = '100%';
            anchor.setAttribute('data-creative-id', creativeId);
            anchor.setAttribute('aria-label', 'Advertisement creative #' + creativeId);

            // 2. Insert creative HTML (or construct <img> fallback)
            if (adData.html) {
                var html = adData.html;
                if (this.apiBase) {
                    html = html.replace(/src="\/([^"]+)"/g, 'src="' + this.apiBase + '/$1"');
                }
                anchor.innerHTML = html;
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
                    clientIP: self.simulatedIP || 'Auto (Real IP)',
                    timestamp: new Date().toISOString()
                };

                // Dispatch custom events for real-time diagnostics (AdPulse & legacy MiniAd)
                window.dispatchEvent(new CustomEvent('adpulse:impression', { detail: eventDetail }));
                window.dispatchEvent(new CustomEvent('miniad:impression', { detail: eventDetail }));
                console.log('[AdPulseTag] Impression tracked for creative #' + creativeId + ' on slot ' + size);
            }

            if (imgElem) {
                imgElem.style.display = 'block';
                imgElem.style.width = '100%';
                imgElem.style.height = '100%';
                imgElem.style.objectFit = 'contain';

                // If relative src and apiBase is set, fix src
                if (self.apiBase && imgElem.getAttribute('src') && imgElem.getAttribute('src').indexOf('http') !== 0) {
                    imgElem.src = self.apiBase + imgElem.getAttribute('src');
                }

                // Error fallback: If external image fails, fallback to reliable local SVG creative
                imgElem.addEventListener('error', function () {
                    var fallbackSvg = (self.apiBase || '') + '/assets/creative_' + size + '.svg';
                    if (imgElem.src !== fallbackSvg) {
                        console.warn('[AdPulseTag] Creative image #' + creativeId + ' failed to load. Switching to local SVG:', fallbackSvg);
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
                var clickDetail = {
                    slotId: slot.id || null,
                    size: size,
                    creativeId: creativeId,
                    clickUrl: clickUrl,
                    clientIP: self.simulatedIP || 'Auto (Real IP)',
                    timestamp: new Date().toISOString()
                };
                window.dispatchEvent(new CustomEvent('adpulse:click', { detail: clickDetail }));
                window.dispatchEvent(new CustomEvent('miniad:click', { detail: clickDetail }));
            });

            // Dispatch ad loaded event
            var loadDetail = {
                slotId: slot.id || null,
                size: size,
                creativeId: creativeId,
                clickUrl: clickUrl,
                impressionUrl: impressionUrl,
                clientIP: self.simulatedIP || 'Auto (Real IP)',
                timestamp: new Date().toISOString()
            };
            window.dispatchEvent(new CustomEvent('adpulse:loaded', { detail: loadDetail }));
            window.dispatchEvent(new CustomEvent('miniad:loaded', { detail: loadDetail }));
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

            var emptyDetail = {
                slotId: slot.id || null,
                size: size,
                status: 204,
                clientIP: this.simulatedIP || 'Auto (Real IP)',
                timestamp: new Date().toISOString()
            };
            window.dispatchEvent(new CustomEvent('adpulse:empty', { detail: emptyDetail }));
            window.dispatchEvent(new CustomEvent('miniad:empty', { detail: emptyDetail }));
            console.log('[AdPulseTag] No ad content available for slot ' + size + ' (HTTP 204)');
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

            var errDetail = {
                slotId: slot.id || null,
                size: size,
                error: error.message,
                timestamp: new Date().toISOString()
            };
            window.dispatchEvent(new CustomEvent('adpulse:error', { detail: errDetail }));
            window.dispatchEvent(new CustomEvent('miniad:error', { detail: errDetail }));
        }
    };

    // Export globally (primary AdPulseTag with MiniAdTag backward compatibility alias)
    window.AdPulseTag = AdPulseTag;
    window.MiniAdTag = AdPulseTag;

    // Auto-initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function () {
            AdPulseTag.init();
        });
    } else {
        AdPulseTag.init();
    }

})(window, document);
