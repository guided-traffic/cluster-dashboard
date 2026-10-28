import { TestBed } from '@angular/core/testing';
import { GroupHead } from './group-head';

describe('GroupHead', () => {
  function render(label: string, count?: number) {
    const fixture = TestBed.createComponent(GroupHead);
    fixture.componentRef.setInput('label', label);
    fixture.componentRef.setInput('count', count);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('heads a group with its name and its count, read as one heading', () => {
    const host = render('monitoring', 4);
    const heading = host.querySelector('h3')!;
    expect(heading.querySelector('.label')!.textContent).toBe('monitoring');
    expect(heading.querySelector('.count')!.textContent).toBe('4');
    expect(heading.textContent!.replace(/\s+/g, ' ').trim()).toBe('monitoring, 4');
  });

  it('names a group without a count', () => {
    const host = render('docs');
    expect(host.querySelector('h3')!.textContent!.trim()).toBe('docs');
    expect(host.querySelector('.count')).toBeNull();
  });
});
