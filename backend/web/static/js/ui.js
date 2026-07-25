/* ===== UI System — Toast, Loading, Confirm ===== */

// ---------- Toast Notifications ----------

(function() {
    // Create container on load
    let container = document.querySelector('.toast-container');
    if (!container) {
        container = document.createElement('div');
        container.className = 'toast-container';
        document.body.appendChild(container);
    }
    window._toastContainer = container;
})();

/**
 * Show a toast notification
 * @param {string} message - Main message text
 * @param {'success'|'error'|'warning'|'info'} type - Toast type
 * @param {object} opts - Options: { title, duration }
 */
function showToast(message, type, opts) {
    type = type || 'info';
    opts = opts || {};
    var duration = opts.duration || 4000;

    var icons = {
        success: '✓',
        error: '✕',
        warning: '⚠',
        info: 'ℹ'
    };

    var toast = document.createElement('div');
    toast.className = 'toast toast-' + type;
    toast.style.setProperty('--toast-duration', duration + 'ms');
    toast.innerHTML =
        '<span class="toast-icon">' + icons[type] + '</span>' +
        '<div class="toast-content">' +
            (opts.title ? '<div class="toast-title">' + escapeHtml(opts.title) + '</div>' : '') +
            '<div class="toast-message">' + escapeHtml(message) + '</div>' +
        '</div>' +
        '<button class="toast-close" onclick="dismissToast(this.parentElement)">&times;</button>';

    // Set animation duration
    toast.querySelector('.toast')
    toast.style.animationDuration = duration + 'ms';
    // Use ::after for progress bar
    var style = document.createElement('style');
    style.textContent = '#' + 'toast-' + Date.now() + '::after { animation-duration: ' + duration + 'ms; }';
    toast.id = 'toast-' + Date.now();
    document.head.appendChild(style);

    window._toastContainer.appendChild(toast);

    // Trigger show animation
    requestAnimationFrame(function() {
        toast.classList.add('show');
    });

    // Auto dismiss
    var timer = setTimeout(function() {
        dismissToast(toast);
    }, duration);

    toast._timer = timer;
}

function dismissToast(toast) {
    if (!toast || toast._dismissed) return;
    toast._dismissed = true;
    clearTimeout(toast._timer);
    toast.classList.remove('show');
    toast.classList.add('hide');
    setTimeout(function() {
        if (toast.parentElement) toast.parentElement.removeChild(toast);
    }, 400);
}

function escapeHtml(str) {
    var div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

// ---------- Button Loading State ----------

/**
 * Set button to loading state
 * @param {HTMLElement} btn - Button element
 * @param {string} loadingText - Optional text while loading
 */
function btnLoading(btn, loadingText) {
    if (!btn) return;
    btn._originalText = btn.innerHTML;
    btn.classList.add('loading');
    btn.disabled = true;
    if (loadingText) btn.textContent = loadingText;
}

function btnReset(btn) {
    if (!btn) return;
    btn.classList.remove('loading');
    btn.disabled = false;
    if (btn._originalText) btn.innerHTML = btn._originalText;
}

// ---------- Confirm Dialog (replaces data-confirm) ----------

document.addEventListener('DOMContentLoaded', function() {
    document.querySelectorAll('[data-confirm]').forEach(function(el) {
        el.addEventListener('click', function(e) {
            if (!confirm(this.dataset.confirm)) {
                e.preventDefault();
            }
        });
    });
});

// ---------- Form Submit with Loading ----------

document.addEventListener('DOMContentLoaded', function() {
    document.querySelectorAll('form[data-loading]').forEach(function(form) {
        form.addEventListener('submit', function() {
            var btn = form.querySelector('button[type="submit"], input[type="submit"]');
            if (btn) btnLoading(btn);
        });
    });
});

// ---------- Smooth Scroll to Error ----------

document.addEventListener('DOMContentLoaded', function() {
    var alert = document.querySelector('.alert-error');
    if (alert) {
        alert.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
});
