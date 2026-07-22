import { Kova } from './index';

// Browser global: exposes `window.Kova` with an `init` method.
(globalThis as any).Kova = Kova;

export { Kova };
