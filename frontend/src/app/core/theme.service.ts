import { DOCUMENT } from '@angular/common';
import { computed, effect, inject, Injectable, signal } from '@angular/core';

/** The person's choice: follow the system, or fix one scheme (docs/adr/0005). */
export type ThemePreference = 'system' | 'light' | 'dark';
export type ColourScheme = 'light' | 'dark';

/** Where the browser keeps the choice; public/theme-init.js reads the same key. */
export const themeStorageKey = 'dash.theme';
export const themePreferences: readonly ThemePreference[] = ['system', 'light', 'dark'];

export function isThemePreference(value: unknown): value is ThemePreference {
  return themePreferences.includes(value as ThemePreference);
}

/**
 * The colour scheme. The preference lives in the browser's storage; it is the data-theme
 * attribute of <html>, which src/app/design/tokens.css turns into the page's color-scheme —
 * `system` leaves it to prefers-color-scheme, live. Storage that throws (a private window, blocked
 * site data) reads as `system`, and a choice it refuses holds for this page.
 */
@Injectable({ providedIn: 'root' })
export class ThemeService {
  private readonly document = inject(DOCUMENT);
  private readonly media = this.document.defaultView?.matchMedia?.('(prefers-color-scheme: dark)');
  private readonly systemDark = signal(this.media?.matches ?? false);

  readonly preference = signal<ThemePreference>(this.stored());
  /** The scheme in effect: the preference, or the system's while the preference is `system`. */
  readonly scheme = computed<ColourScheme>(() => {
    const preference = this.preference();
    if (preference === 'system') {
      return this.systemDark() ? 'dark' : 'light';
    }
    return preference;
  });

  constructor() {
    this.media?.addEventListener('change', (event) => this.systemDark.set(event.matches));
    this.apply(this.preference());
    effect(() => this.apply(this.preference()));
  }

  set(preference: ThemePreference): void {
    this.preference.set(preference);
    try {
      this.document.defaultView?.localStorage.setItem(themeStorageKey, preference);
    } catch {
      // Storage refused (private mode, quota, blocked site data): the choice holds for this page.
    }
  }

  private stored(): ThemePreference {
    try {
      const value = this.document.defaultView?.localStorage.getItem(themeStorageKey);
      return isThemePreference(value) ? value : 'system';
    } catch {
      return 'system';
    }
  }

  private apply(preference: ThemePreference): void {
    this.document.documentElement.setAttribute('data-theme', preference);
  }
}
