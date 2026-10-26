import { TestBed } from '@angular/core/testing';
import { Summary as SummaryModel } from '../../api/models/summary';
import { Summary, SummaryToken } from './summary';

describe('Summary', () => {
  function render(summary: SummaryModel, tokens: SummaryToken[]) {
    const fixture = TestBed.createComponent(Summary);
    fixture.componentRef.setInput('summary', summary);
    fixture.componentRef.setInput('tokens', tokens);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('draws a token per app linking to its tile, and names only what is not healthy', () => {
    const host = render({ total: 4, healthy: 1, degraded: 1, down: 1, unknown: 1 }, [
      { name: 'Grafana', state: 'healthy', anchor: 'app-0' },
      { name: 'Prometheus', state: 'degraded', anchor: 'app-1' },
      { name: 'Immich', state: 'down', anchor: 'app-2' },
      { name: 'Router', state: 'unknown', anchor: 'app-3' },
    ]);
    expect(host.querySelector('app-bracket h2')!.textContent!.replace(/\s+/g, ' ').trim()).toBe(
      'apps ·, 4',
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
    const parts = [...host.querySelectorAll('.legend .part')].map((p) => p.textContent!.trim());
    expect(parts).toEqual(['1 down', '1 degraded', '1 unknown']);
    expect(host.querySelectorAll('.legend .visually-hidden').length).toBe(2);
  });

  it('says all healthy when nothing lights up', () => {
    const host = render({ total: 1, healthy: 1, degraded: 0, down: 0, unknown: 0 }, [
      { name: 'Grafana', state: 'healthy', anchor: 'app-0' },
    ]);
    expect(host.querySelector('.legend')!.textContent!.trim()).toBe('all healthy');
  });
});
