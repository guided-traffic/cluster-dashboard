import { TestBed } from '@angular/core/testing';
import { BookmarkGroup } from '../../api/models/bookmark-group';
import { Bookmarks } from './bookmarks';

const groups: BookmarkGroup[] = [
  {
    name: 'docs',
    bookmarks: [
      {
        id: 'bookmark/0/0',
        name: 'Kubernetes',
        url: 'https://kubernetes.io/docs',
        target_blank: true,
        icon: { src: '/k.svg', tint: true },
        badges: [{ corner: 'top-right', tone: 'cyan', label: 'external', text: 'ext' }],
      },
      {
        id: 'bookmark/0/1',
        name: 'Helm',
        url: 'https://helm.sh',
        target_blank: false,
        icon: { src: '/h.svg', tint: true },
        badges: [],
      },
    ],
  },
];

describe('Bookmarks', () => {
  function render(showGroups: boolean) {
    const fixture = TestBed.createComponent(Bookmarks);
    fixture.componentRef.setInput('groups', groups);
    fixture.componentRef.setInput('showGroups', showGroups);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('lists each group under its bracket, badges as chips after the name', () => {
    const host = render(true);
    expect(host.querySelector('app-bracket h2')!.textContent!.replace(/\s+/g, ' ').trim()).toBe(
      'docs ·, 2',
    );
    const links = [...host.querySelectorAll('a')];
    expect(
      links.map((a) => [a.getAttribute('href'), a.getAttribute('target'), a.getAttribute('rel')]),
    ).toEqual([
      ['https://kubernetes.io/docs', '_blank', 'noopener noreferrer'],
      ['https://helm.sh', null, null],
    ]);
    expect(links[0].querySelector('app-badge')!.classList.contains('chip')).toBe(true);
    expect(links[0].querySelector('app-item-icon')!.getAttribute('style')).toContain(
      '--icon-size: 20px',
    );
  });

  it('lists without heads while ui.showBookmarkGroups is off', () => {
    const host = render(false);
    expect(host.querySelector('app-bracket')).toBeNull();
    expect(host.querySelectorAll('li').length).toBe(2);
  });
});
