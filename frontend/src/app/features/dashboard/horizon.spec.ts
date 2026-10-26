import { TestBed } from '@angular/core/testing';
import { Horizon, horizonLines } from './horizon';

describe('Horizon', () => {
  it('draws twelve rows closing in towards the horizon and twenty-five rays', () => {
    const lines = horizonLines();
    expect(lines.length).toBe(37);
    const rows = lines.slice(0, 12).map((l) => l.y1);
    expect(rows[0]).toBe(360);
    for (let i = 1; i < rows.length; i++) {
      expect(rows[i]).toBeLessThan(rows[i - 1]);
      expect(rows[i - 1] - rows[i]).toBeLessThan(i === 1 ? 1000 : rows[i - 2] - rows[i - 1]);
    }
    const rays = lines.slice(12);
    expect(rays.every((r) => r.y2 === 360)).toBe(true);
    expect(rays[12].x1).toBe(500);
  });

  it('is decorative behind what it holds', () => {
    const fixture = TestBed.createComponent(Horizon);
    fixture.detectChanges();
    const host = fixture.nativeElement as HTMLElement;
    expect(host.querySelector('svg')!.getAttribute('aria-hidden')).toBe('true');
    expect(host.querySelectorAll('line').length).toBe(37);
  });
});
