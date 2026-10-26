import { TestBed } from '@angular/core/testing';
import { ThemeService } from '../../core/theme.service';
import { ItemIcon } from './item-icon';
import { Icon } from '../../api/models/icon';

describe('ItemIcon', () => {
  afterEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
  });

  function render(icon: Icon, size?: number) {
    const fixture = TestBed.createComponent(ItemIcon);
    fixture.componentRef.setInput('icon', icon);
    if (size !== undefined) {
      fixture.componentRef.setInput('size', size);
    }
    fixture.detectChanges();
    return { fixture, host: fixture.nativeElement as HTMLElement };
  }

  it('draws a glyph as a mask in the text colour', () => {
    const { host } = render({ src: '/api/v1/icons/mdi@1.2.3/web.svg', tint: true });
    const glyph = host.querySelector<HTMLElement>('.glyph')!;
    expect(glyph.style.getPropertyValue('--mask')).toBe('url("/api/v1/icons/mdi@1.2.3/web.svg")');
    expect(host.querySelector('img')).toBeNull();
    expect(host.getAttribute('aria-hidden')).toBe('true');
    expect(host.style.getPropertyValue('--icon-size')).toBe('40px');
  });

  it('draws a logo as an image, decorative, at its size', () => {
    const { host } = render({ src: '/api/v1/icons/sh@abc/grafana.svg', tint: false }, 20);
    const img = host.querySelector('img')!;
    expect(img.getAttribute('src')).toBe('/api/v1/icons/sh@abc/grafana.svg');
    expect(img.getAttribute('alt')).toBe('');
    expect(host.style.getPropertyValue('--icon-size')).toBe('20px');
  });

  it('shows the dark variant in the dark scheme, and follows a change of scheme', () => {
    const theme = TestBed.inject(ThemeService);
    theme.set('dark');
    const { fixture, host } = render({ src: '/a.svg', src_dark: '/a-light.svg', tint: false });
    expect(host.querySelector('img')!.getAttribute('src')).toBe('/a-light.svg');
    theme.set('light');
    fixture.detectChanges();
    expect(host.querySelector('img')!.getAttribute('src')).toBe('/a.svg');
  });

  it('keeps the one source of an icon without a dark variant', () => {
    TestBed.inject(ThemeService).set('dark');
    const { host } = render({ src: '/a.svg', tint: false });
    expect(host.querySelector('img')!.getAttribute('src')).toBe('/a.svg');
  });
});
