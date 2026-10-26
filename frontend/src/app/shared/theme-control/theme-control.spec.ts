import { TestBed } from '@angular/core/testing';
import { ThemeService, themeStorageKey } from '../../core/theme.service';
import { ThemeControl } from './theme-control';

describe('ThemeControl', () => {
  function render() {
    const fixture = TestBed.createComponent(ThemeControl);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    const radios = () => Array.from(root.querySelectorAll<HTMLInputElement>('input[type=radio]'));
    return { fixture, root, radios };
  }

  afterEach(() => {
    document.documentElement.removeAttribute('data-theme');
    localStorage.clear();
  });

  it('is a named group of three segments: system, light, dark', () => {
    const { root, radios } = render();
    expect(root.querySelector('legend')?.textContent?.trim()).toBe('Colour scheme');
    expect(radios().map((radio) => radio.value)).toEqual(['system', 'light', 'dark']);
    const names = Array.from(root.querySelectorAll('label')).map((label) => label.textContent?.trim());
    expect(names).toEqual(['System', 'Light', 'Dark']);
  });

  it('checks the segment of the stored preference', () => {
    localStorage.setItem(themeStorageKey, 'dark');
    const { radios } = render();
    expect(radios().find((radio) => radio.checked)?.value).toBe('dark');
  });

  it('checks system when nothing is stored', () => {
    const { radios } = render();
    expect(radios().find((radio) => radio.checked)?.value).toBe('system');
  });

  it('stores the segment a person picks and moves the check', async () => {
    const { fixture, radios } = render();
    radios()[1].click();
    await fixture.whenStable();
    expect(TestBed.inject(ThemeService).preference()).toBe('light');
    expect(localStorage.getItem(themeStorageKey)).toBe('light');
    expect(radios().find((radio) => radio.checked)?.value).toBe('light');
  });

  it('draws every glyph hidden from screen readers', () => {
    const { root } = render();
    const svgs = Array.from(root.querySelectorAll('svg'));
    expect(svgs).toHaveLength(3);
    svgs.forEach((svg) => expect(svg.getAttribute('aria-hidden')).toBe('true'));
    svgs.forEach((svg) => expect(svg.querySelector('path')?.getAttribute('d')).toBeTruthy());
  });
});
