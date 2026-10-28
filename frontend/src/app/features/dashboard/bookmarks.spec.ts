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
  {
    name: 'mail',
    bookmarks: [
      {
        id: 'bookmark/1/0',
        name: 'Mail',
        url: 'https://mail.example.com/inbox',
        target_blank: false,
        icon: { src: '/m.svg', tint: true },
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

  const text = (el: Element | null) => el?.textContent?.replace(/\s+/g, ' ').trim();

  it('lists each group under its head, badges as chips after the name', () => {
    const host = render(true);
    expect([...host.querySelectorAll('app-group-head h3')].map(text)).toEqual([
      'docs, 2',
      'mail, 1',
    ]);
    const links = [...host.querySelectorAll('a')];
    expect(
      links.map((a) => [a.getAttribute('href'), a.getAttribute('target'), a.getAttribute('rel')]),
    ).toEqual([
      ['https://kubernetes.io/docs', '_blank', 'noopener noreferrer'],
      ['https://helm.sh', null, null],
      ['https://mail.example.com/inbox', null, null],
    ]);
    expect(links[0].querySelector('app-badge')!.classList.contains('chip')).toBe(true);
    expect(links[0].querySelector('app-item-icon')!.getAttribute('style')).toContain(
      '--icon-size: 20px',
    );
  });

  it('numbers the rows across the groups and shows each address, the number hidden from screen readers', () => {
    const host = render(true);
    const links = [...host.querySelectorAll('a')];
    expect(links.map((a) => text(a.querySelector('.index')))).toEqual(['01', '02', '03']);
    expect(links[0].querySelector('.index')!.getAttribute('aria-hidden')).toBe('true');
    expect(links.map((a) => text(a.querySelector('.url')))).toEqual([
      'kubernetes.io/docs',
      'helm.sh',
      'mail.example.com/inbox',
    ]);
    expect(links[0].querySelector('.arrow')!.getAttribute('aria-hidden')).toBe('true');
  });

  it('lists without heads while ui.showBookmarkGroups is off', () => {
    const host = render(false);
    expect(host.querySelector('app-group-head')).toBeNull();
    expect(host.querySelectorAll('li').length).toBe(3);
  });
});
