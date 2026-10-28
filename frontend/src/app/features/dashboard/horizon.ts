import { ChangeDetectionStrategy, Component } from '@angular/core';

/** The floor's share of the height, and the grid's rows and rays (after Cogpit's glow.go). */
const width = 1000;
const height = 360;
const floor = 0.42;
const rows = 12;
const rays = 25;

/** The grid floor's lines: rows closing in towards the horizon, rays meeting above its middle. */
export function horizonLines(): { x1: number; y1: number; x2: number; y2: number }[] {
  const top = height * (1 - floor);
  const depth = height - top;
  const lines = [];
  for (let i = 0; i < rows; i++) {
    const y = top + depth / (1 + i * 0.55);
    lines.push({ x1: 0, y1: y, x2: width, y2: y });
  }
  const vx = width / 2;
  const vy = top - depth * 0.6;
  for (let i = 0; i < rays; i++) {
    const f = (i / (rays - 1)) * 2 - 1;
    const bx = vx + f * width * 1.6;
    const t = (top - vy) / (height - vy);
    lines.push({ x1: vx + (bx - vx) * t, y1: top, x2: bx, y2: height });
  }
  return lines.map((l) => ({
    x1: Math.round(l.x1 * 10) / 10,
    y1: Math.round(l.y1 * 10) / 10,
    x2: Math.round(l.x2 * 10) / 10,
    y2: Math.round(l.y2 * 10) / 10,
  }));
}

/**
 * The horizon (docs/adr/0012 D6): a perspective grid floor of hairlines that fades towards the
 * horizon. Only where there is no data — the empty dashboard, a backend that does not answer —
 * never behind tiles. What the page says there is its content.
 */
@Component({
  selector: 'app-horizon',
  template: `
    <svg
      class="floor"
      viewBox="0 0 1000 360"
      preserveAspectRatio="none"
      aria-hidden="true"
      focusable="false"
    >
      <defs>
        <linearGradient id="horizon-fade" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0.55" stop-color="white" stop-opacity="0" />
          <stop offset="1" stop-color="white" stop-opacity="1" />
        </linearGradient>
        <mask id="horizon-mask">
          <rect width="1000" height="360" fill="url(#horizon-fade)" />
        </mask>
      </defs>
      <g mask="url(#horizon-mask)">
        @for (l of lines; track $index) {
          <line [attr.x1]="l.x1" [attr.y1]="l.y1" [attr.x2]="l.x2" [attr.y2]="l.y2" />
        }
      </g>
    </svg>
    <div class="content"><ng-content /></div>
  `,
  styleUrl: './horizon.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Horizon {
  protected readonly lines = horizonLines();
}
