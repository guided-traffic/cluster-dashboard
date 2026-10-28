import { TestBed } from '@angular/core/testing';
import { appOf } from '../../../testing/dashboard';
import { App } from '../../api/models/app';
import { shownURL, statusWord, Tile } from './tile';

describe('Tile', () => {
  function render(app: App, showURL = true) {
    const fixture = TestBed.createComponent(Tile);
    fixture.componentRef.setInput('app', app);
    fixture.componentRef.setInput('anchor', 'app-3');
    fixture.componentRef.setInput('showURL', showURL);
    fixture.detectChanges();
    const host = fixture.nativeElement as HTMLElement;
    return { host, link: host.querySelector('a')! };
  }

  it('links the app, quiet while it is healthy: the ready count, no colour', () => {
    const { host, link } = render(
      appOf({ info: 'Dashboards', status: { state: 'healthy', ready: 2, total: 2 } }),
    );
    expect(link.getAttribute('href')).toBe('https://grafana.example.com/');
    expect(link.id).toBe('app-3');
    expect(link.getAttribute('target')).toBeNull();
    expect(link.getAttribute('rel')).toBeNull();
    expect(host.querySelector('.name')!.textContent).toBe('Grafana');
    expect(host.querySelector('.url')!.textContent).toBe('grafana.example.com');
    expect(host.querySelector('.url svg')!.getAttribute('aria-hidden')).toBe('true');
    expect(host.querySelector('.info')!.textContent).toBe('Dashboards');
    expect(host.querySelector('.status')!.textContent!.trim()).toBe('2/2');
    expect(link.classList.contains('degraded') || link.classList.contains('down')).toBe(false);
    expect(host.querySelector('app-item-icon')).not.toBeNull();
  });

  it('opens in a new tab without handing the page over', () => {
    const { link } = render(appOf({ target_blank: true }));
    expect(link.getAttribute('target')).toBe('_blank');
    expect(link.getAttribute('rel')).toBe('noopener noreferrer');
  });

  it('says a degraded and a down app in words, not only in colour', () => {
    let { host, link } = render(appOf({ status: { state: 'degraded', ready: 2, total: 3 } }));
    expect(link.classList.contains('degraded')).toBe(true);
    expect(host.querySelector('.status')!.textContent!.trim()).toBe('2 of 3 ready');
    ({ host, link } = render(appOf({ status: { state: 'down', ready: 0, total: 1 } })));
    expect(link.classList.contains('down')).toBe(true);
    expect(host.querySelector('.status')!.textContent!.trim()).toBe('down');
    ({ host } = render(appOf({ status: { state: 'unknown', ready: 0, total: 0 } })));
    expect(host.querySelector('.status')).toBeNull();
  });

  it('hides the address while ui.showAppUrls is off', () => {
    const { host } = render(appOf(), false);
    expect(host.querySelector('.url')).toBeNull();
  });

  it('puts the badges on the icon', () => {
    const { host } = render(
      appOf({
        badges: [
          {
            corner: 'top-right',
            tone: 'cyan',
            label: 'reachable worldwide',
            icon: { src: '/g.svg', tint: true },
          },
          { corner: 'bottom-left', tone: 'amber', label: 'beta release', text: 'beta' },
        ],
      }),
    );
    const badges = host.querySelectorAll('.badges app-badge');
    expect(badges.length).toBe(2);
    expect(badges[1].getAttribute('aria-label')).toBe('beta release');
    // The badges come after the text: the link's name begins with the app's.
    expect(host.querySelector('a')!.lastElementChild!.classList.contains('badges')).toBe(true);
    expect(host.querySelector('a')!.textContent!.trim().startsWith('Grafana')).toBe(true);
    expect(host.querySelector('app-item-icon')!.getAttribute('style')).toContain(
      '--icon-size: 28px',
    );
  });

  it('shows an address as host and path', () => {
    expect(shownURL('https://grafana.example.com/')).toBe('grafana.example.com');
    expect(shownURL('http://10.0.0.7:3000/d/home')).toBe('10.0.0.7:3000/d/home');
    expect(shownURL('not a url')).toBe('not a url');
    expect(statusWord(appOf())).toBe('1/1');
    expect(statusWord(appOf({ status: { state: 'unknown', ready: 0, total: 0 } }))).toBeUndefined();
  });
});
