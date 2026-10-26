import themeInit from '../../../public/theme-init.js' with { loader: 'text' };
import { themeStorageKey } from './theme.service';

/** Runs public/theme-init.js as the browser does, a classic script in the page. */
function runThemeInit(): void {
  new Function(themeInit)();
}

describe('theme-init.js', () => {
  const html = () => document.documentElement;

  afterEach(() => {
    html().removeAttribute('data-theme');
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it('reads the key the theme service writes', () => {
    expect(themeInit).toContain(`'${themeStorageKey}'`);
  });

  it.each(['system', 'light', 'dark'])(
    'applies a stored %s before the app starts',
    (preference) => {
      localStorage.setItem(themeStorageKey, preference);
      runThemeInit();
      expect(html().getAttribute('data-theme')).toBe(preference);
    },
  );

  it('applies system when nothing is stored', () => {
    runThemeInit();
    expect(html().getAttribute('data-theme')).toBe('system');
  });

  it.each(['purple', '', 'DARK', ' dark'])('applies system for the stored value %j', (value) => {
    localStorage.setItem(themeStorageKey, value);
    runThemeInit();
    expect(html().getAttribute('data-theme')).toBe('system');
  });

  it('applies system when the storage throws', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new DOMException('denied', 'SecurityError');
    });
    expect(() => runThemeInit()).not.toThrow();
    expect(html().getAttribute('data-theme')).toBe('system');
  });

  it('stays below a kilobyte', () => {
    expect(new TextEncoder().encode(themeInit).length).toBeLessThan(1024);
  });
});
