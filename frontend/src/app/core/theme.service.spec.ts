import { TestBed } from '@angular/core/testing';
import { ThemeService, themeStorageKey } from './theme.service';

type ChangeListener = (event: { matches: boolean }) => void;

/** A prefers-color-scheme query the test can flip, as the OS does when its appearance changes. */
function fakeColourScheme(dark: boolean) {
  const listeners: ChangeListener[] = [];
  const query = {
    matches: dark,
    addEventListener: (_type: string, listener: ChangeListener) => listeners.push(listener),
  };
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: vi.fn(() => query),
  });
  return {
    flip(nowDark: boolean) {
      query.matches = nowDark;
      listeners.forEach((listener) => listener({ matches: nowDark }));
    },
  };
}

describe('ThemeService', () => {
  const html = () => document.documentElement;

  function create(): ThemeService {
    return TestBed.inject(ThemeService);
  }

  afterEach(() => {
    html().removeAttribute('data-theme');
    localStorage.clear();
    vi.restoreAllMocks();
    Reflect.deleteProperty(window, 'matchMedia');
  });

  it('starts with system when nothing is stored', () => {
    const theme = create();
    expect(theme.preference()).toBe('system');
    expect(html().getAttribute('data-theme')).toBe('system');
  });

  it.each(['light', 'dark', 'system'] as const)('starts with a stored %s', (preference) => {
    localStorage.setItem(themeStorageKey, preference);
    const theme = create();
    expect(theme.preference()).toBe(preference);
    expect(html().getAttribute('data-theme')).toBe(preference);
  });

  it.each(['purple', '', 'DARK'])('reads the stored value %j as system', (value) => {
    localStorage.setItem(themeStorageKey, value);
    expect(create().preference()).toBe('system');
  });

  it('reads system when the storage throws', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked site data');
    });
    expect(create().preference()).toBe('system');
  });

  it('applies the installation default while nothing is chosen', () => {
    const theme = create();
    theme.applyDefault('dark');
    TestBed.tick();
    expect(theme.preference()).toBe('dark');
    expect(html().getAttribute('data-theme')).toBe('dark');
    expect(localStorage.getItem(themeStorageKey)).toBeNull();
    theme.applyDefault('light');
    expect(theme.preference()).toBe('light'); // a later default too, while nothing is chosen
  });

  it('keeps a stored or a made choice over the installation default', () => {
    localStorage.setItem(themeStorageKey, 'system');
    const stored = create();
    stored.applyDefault('dark');
    expect(stored.preference()).toBe('system');

    TestBed.resetTestingModule();
    localStorage.clear();
    const fresh = create();
    fresh.set('light');
    fresh.applyDefault('dark');
    expect(fresh.preference()).toBe('light');
  });

  it('stores a choice and applies it to <html>', () => {
    const theme = create();
    theme.set('dark');
    TestBed.tick();
    expect(theme.preference()).toBe('dark');
    expect(localStorage.getItem(themeStorageKey)).toBe('dark');
    expect(html().getAttribute('data-theme')).toBe('dark');
  });

  it('keeps a choice for the page when the storage refuses it', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota exceeded');
    });
    const theme = create();
    expect(() => theme.set('light')).not.toThrow();
    TestBed.tick();
    expect(theme.preference()).toBe('light');
    expect(html().getAttribute('data-theme')).toBe('light');
  });

  it('follows the system scheme live while the preference is system', () => {
    const os = fakeColourScheme(false);
    const theme = create();
    expect(theme.scheme()).toBe('light');
    os.flip(true);
    expect(theme.scheme()).toBe('dark');
    os.flip(false);
    expect(theme.scheme()).toBe('light');
  });

  it('ignores the system scheme once a scheme is chosen', () => {
    const os = fakeColourScheme(true);
    const theme = create();
    expect(theme.scheme()).toBe('dark');
    theme.set('light');
    os.flip(true);
    expect(theme.scheme()).toBe('light');
  });

  it('reads light when the browser has no matchMedia', () => {
    expect(create().scheme()).toBe('light');
  });
});
