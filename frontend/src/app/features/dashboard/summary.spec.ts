import { TestBed } from '@angular/core/testing';
import { Summary as SummaryModel } from '../../api/models/summary';
import { leadWords, Summary, SummaryToken } from './summary';

describe('Summary', () => {
  function render(summary: SummaryModel, tokens: SummaryToken[], groups?: number) {
    const fixture = TestBed.createComponent(Summary);
    fixture.componentRef.setInput('summary', summary);
    fixture.componentRef.setInput('tokens', tokens);
    fixture.componentRef.setInput('groups', groups);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  const text = (el: Element | null) => el?.textContent?.replace(/\s+/g, ' ').trim();

  it('draws a square per app linking to its tile, and names what is not healthy, worst first', () => {
    const host = render(
      { total: 4, healthy: 1, degraded: 1, down: 1, unknown: 1 },
      [
        { name: 'Grafana', state: 'healthy', anchor: 'app-0' },
        { name: 'Prometheus', state: 'degraded', anchor: 'app-1' },
        { name: 'Immich', state: 'down', anchor: 'app-2' },
        { name: 'Router', state: 'unknown', anchor: 'app-3' },
      ],
      2,
    );
    const tokens = [...host.querySelectorAll<HTMLAnchorElement>('.token')];
    expect(
      tokens.map((t) => [
        t.getAttribute('href'),
        [...t.classList].find((c) => c !== 'token'),
        t.title,
      ]),
    ).toEqual([
      ['#app-0', 'healthy', 'Grafana'],
      ['#app-1', 'degraded', 'Prometheus'],
      ['#app-2', 'down', 'Immich'],
      ['#app-3', 'unknown', 'Router'],
    ]);
    expect(host.querySelector('.tokens')!.getAttribute('aria-hidden')).toBe('true');
    expect(tokens[0].getAttribute('tabindex')).toBe('-1');
    expect(text(host.querySelector('.legend'))).toBe(
      '4 apps in 2 groups. 1 down, 1 degraded, 1 unknown.',
    );
    const parts = [...host.querySelectorAll('.legend .part')].map((p) =>
      [...p.classList].find((c) => c !== 'part'),
    );
    expect(parts).toEqual(['down', 'degraded', 'unknown']);
  });

  it('says all healthy when nothing lights up', () => {
    const host = render({ total: 1, healthy: 1, degraded: 0, down: 0, unknown: 0 }, [
      { name: 'Grafana', state: 'healthy', anchor: 'app-0' },
    ]);
    expect(text(host.querySelector('.legend'))).toBe('1 app. All healthy.');
  });

  it('counts every state in a list a screen reader reads, a zero drawn faint', () => {
    const host = render({ total: 5, healthy: 3, degraded: 0, down: 2, unknown: 0 }, []);
    const counts = [...host.querySelectorAll('dl.counts > div')];
    expect(counts.map((c) => [text(c.querySelector('dt')), text(c.querySelector('dd'))])).toEqual([
      ['Apps', '5'],
      ['Healthy', '3'],
      ['Degraded', '0'],
      ['Down', '2'],
      ['Unknown', '0'],
    ]);
    expect(counts.map((c) => c.classList.contains('zero'))).toEqual([
      false,
      false,
      true,
      false,
      true,
    ]);
  });

  it('words the apps and their groups', () => {
    expect(leadWords(13, 3)).toBe('13 apps in 3 groups.');
    expect(leadWords(1, 1)).toBe('1 app in 1 group.');
    expect(leadWords(4)).toBe('4 apps.');
  });
});
