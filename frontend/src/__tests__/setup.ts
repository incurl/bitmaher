/**
 * @fileoverview Global test setup — polyfills for APIs missing in happy-dom.
 *
 * happy-dom does not implement the Web Animations API (Element.prototype.animate).
 * Libraries like @formkit/auto-animate call el.animate() at runtime, which throws
 * "el.animate is not a function" in the test environment. A no-op stub satisfies
 * the call without affecting test semantics.
 */

if (typeof Element.prototype.animate !== 'function') {
  Element.prototype.animate = function (): Animation {
    return {
      onfinish: null,
      cancel: () => {},
      finish: () => {},
      play: () => {},
      pause: () => {},
      reverse: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    } as unknown as Animation
  }
}
