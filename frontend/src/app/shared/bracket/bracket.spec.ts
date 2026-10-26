import { TestBed } from '@angular/core/testing';
import { Bracket } from './bracket';

describe('Bracket', () => {
  it('heads a group with its label and count, the rule decorative', () => {
    const fixture = TestBed.createComponent(Bracket);
    fixture.componentRef.setInput('label', 'monitoring');
    fixture.componentRef.setInput('count', 4);
    fixture.detectChanges();
    const host = fixture.nativeElement as HTMLElement;
    expect(host.querySelector('h2')!.textContent!.replace(/\s+/g, ' ').trim()).toBe(
      'monitoring ·, 4',
    );
    expect(host.querySelector('.rule')!.getAttribute('aria-hidden')).toBe('true');
    expect(host.classList.contains('muted')).toBe(false);
  });

  it('names a group without a count, in the muted light', () => {
    const fixture = TestBed.createComponent(Bracket);
    fixture.componentRef.setInput('label', 'docs');
    fixture.componentRef.setInput('tone', 'muted');
    fixture.detectChanges();
    const host = fixture.nativeElement as HTMLElement;
    expect(host.querySelector('h2')!.textContent!.trim()).toBe('docs');
    expect(host.classList.contains('muted')).toBe(true);
  });
});
