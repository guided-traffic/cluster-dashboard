import { TestBed } from '@angular/core/testing';
import { Badge as BadgeModel } from '../../api/models/badge';
import { Badge } from './badge';

describe('Badge', () => {
  function render(badge: BadgeModel, chip = false) {
    const fixture = TestBed.createComponent(Badge);
    fixture.componentRef.setInput('badge', badge);
    fixture.componentRef.setInput('chip', chip);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('draws a glyph badge at its corner in its tone, named by its label', () => {
    const host = render({
      corner: 'top-right',
      tone: 'cyan',
      label: 'reachable worldwide',
      icon: { src: '/api/v1/icons/material-symbols@1/public.svg', tint: true },
    });
    expect(host.classList.value.split(' ').sort()).toEqual([
      'corner',
      'corner-top-right',
      'tone-cyan',
    ]);
    expect(host.getAttribute('role')).toBe('img');
    expect(host.getAttribute('aria-label')).toBe('reachable worldwide');
    expect(host.getAttribute('title')).toBe('reachable worldwide');
    expect(host.querySelector<HTMLElement>('.glyph')!.style.getPropertyValue('--mask')).toBe(
      'url("/api/v1/icons/material-symbols@1/public.svg")',
    );
  });

  it('draws a text badge as text, never as markup', () => {
    const host = render({
      corner: 'bottom-left',
      tone: 'amber',
      label: 'beta release',
      text: '<b>beta',
    });
    expect(host.classList.contains('corner-bottom-left')).toBe(true);
    expect(host.querySelector('.text')!.textContent).toBe('<b>beta');
    expect(host.querySelector('b')).toBeNull();
  });

  it('becomes a chip after a name', () => {
    const host = render(
      { corner: 'top-right', tone: 'neutral', label: 'docs', text: 'docs' },
      true,
    );
    expect(host.classList.value.split(' ').sort()).toEqual(['chip', 'tone-neutral']);
  });
});
