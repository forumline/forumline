/*
 * Toast & Error Banner Notifications
 *
 * Success/info: brief ephemeral toast (3s auto-dismiss, bottom-right).
 * Error: persistent banner at top of page. First error wins — stays until
 * dismissed. Dismissing allows the next error to claim the slot.
 */

let toastId = 0;
let errorBannerVisible = false;

export function toast(message, type = 'info') {
  const container = document.getElementById('toast-container');
  if (!container) return;

  const id = ++toastId;
  const el = document.createElement('div');
  el.id = `toast-${id}`;
  el.className = `px-4 py-3 rounded-lg shadow-lg text-sm font-medium transform transition-all duration-300 translate-x-full ${
    type === 'success'
      ? 'bg-green-600 text-white'
      : 'bg-slate-700 text-slate-100'
  }`;
  el.textContent = message;
  container.appendChild(el);

  requestAnimationFrame(() => {
    el.classList.remove('translate-x-full');
  });

  setTimeout(() => {
    el.classList.add('translate-x-full');
    setTimeout(() => el.remove(), 300);
  }, 3000);
}

toast.success = msg => toast(msg, 'success');

toast.error = msg => {
  if (errorBannerVisible) return;
  errorBannerVisible = true;

  let banner = document.getElementById('error-banner');
  if (!banner) {
    banner = document.createElement('div');
    banner.id = 'error-banner';
    banner.className = 'fixed top-0 left-0 right-0 z-50 flex items-center gap-2 px-4 py-2 bg-red-600 text-white text-sm font-medium animate-slide-down';

    const icon = document.createElement('span');
    icon.textContent = '\u26A0';
    icon.className = 'flex-shrink-0';

    const text = document.createElement('span');
    text.id = 'error-banner-text';
    text.className = 'flex-1';
    text.textContent = msg;

    const close = document.createElement('button');
    close.textContent = '\u00D7';
    close.className = 'text-lg leading-none opacity-70 hover:opacity-100 cursor-pointer';
    close.setAttribute('aria-label', 'Dismiss error');
    close.addEventListener('click', () => {
      banner.remove();
      errorBannerVisible = false;
    });

    banner.append(icon, text, close);
    document.body.prepend(banner);
  } else {
    document.getElementById('error-banner-text').textContent = msg;
    banner.classList.remove('hidden');
  }
};

export function hideErrorBanner() {
  const banner = document.getElementById('error-banner');
  if (banner) {
    banner.remove();
    errorBannerVisible = false;
  }
}
