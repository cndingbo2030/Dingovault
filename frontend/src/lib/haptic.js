/** Best-effort light vibration (Android WebView); no-op elsewhere. */
export function hapticLight() {
  try {
    const w = typeof window !== 'undefined' ? window : undefined
    const b = w && /** @type {any} */ (w).AndroidBridge
    if (b && typeof b.vibrateShort === 'function') {
      b.vibrateShort()
    }
  } catch {
    /* ignore */
  }
}
