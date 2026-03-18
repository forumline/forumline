import { $ } from '../lib/utils.js';

let _visible = false;

export function showErrorBanner(message) {
  if (_visible) return;
  const banner = $('errorBanner');
  if (!banner) return;
  _visible = true;
  $('errorBannerMessage').textContent = message;
  banner.classList.remove('hidden');
}

export function hideErrorBanner() {
  const banner = $('errorBanner');
  if (!banner) return;
  _visible = false;
  banner.classList.add('hidden');
}

export function isErrorBannerVisible() {
  return _visible;
}
