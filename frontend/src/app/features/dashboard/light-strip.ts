import {
  AfterViewInit,
  ChangeDetectionStrategy,
  Component,
  computed,
  DestroyRef,
  ElementRef,
  inject,
  input,
} from '@angular/core';
import { LightStrip as LightStripSettings } from '../../api/models/light-strip';

/** The colours ui.lightStrip.color takes by name, each a token --strip-<name> in tokens.css. */
export const lightStripColors = ['cyan', 'blue', 'magenta', 'red', 'orange', 'yellow', 'green'];

/** The light's colour as CSS: a name's token, a #rrggbb as given; undefined for none, the glass
 *  alone. Anything else is none too: the backend refused it already, this is the second line. */
export function lightOf(color: string): string | undefined {
  if (lightStripColors.includes(color)) {
    return `var(--strip-${color})`;
  }
  return /^#[0-9a-fA-F]{6}$/.test(color) ? color : undefined;
}

/** The grain's texture, Cogpit's (lightstrip_model.go stripNoise): 256 × 64 specks. */
const grainW = 256;
const grainH = 64;

/**
 * The grain's texture as RGBA, row by row: each speck black or white at random, its alpha
 * (2|r − ½|)² at the strength Cogpit draws it with (10 %, times its lab's 0.6), from a linear
 * congruential generator seeded with 7 — the same specks at every start and in both schemes.
 */
export function grainTexture(): Uint8ClampedArray<ArrayBuffer> {
  const px = new Uint8ClampedArray(grainW * grainH * 4);
  let s = 7;
  for (let i = 0; i < px.length; i += 4) {
    s = (Math.imul(s, 1664525) + 1013904223) >>> 0;
    const r = s / 4294967296;
    const v = r >= 0.5 ? 255 : 0;
    const a = Math.abs(r - 0.5) * 2;
    px[i] = px[i + 1] = px[i + 2] = v;
    px[i + 3] = Math.round(Math.round(a * a * 255) * 0.1 * 0.6);
  }
  return px;
}

/** The texture tiled over w × h device pixels, a speck per pixel. */
export function tileGrain(w: number, h: number): Uint8ClampedArray<ArrayBuffer> {
  const texture = grainTexture();
  const px = new Uint8ClampedArray(w * h * 4);
  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const from = ((y % grainH) * grainW + (x % grainW)) * 4;
      px.set(texture.subarray(from, from + 4), (y * w + x) * 4);
    }
  }
  return px;
}

/**
 * The light strip (Cogpit's D-122, docs/adr/0013): a strip of frosted glass over the greeting,
 * lit in the colour ui.lightStrip.color names, so the environment shows at a glance. The light
 * stands still or breathes, scans or flows; it fades in from the glass and starts from its
 * beginning whenever its colour or animation changes, and stands still where the browser asks for
 * reduced motion. The grain lies over the light, a speck per device pixel. Decorative: the
 * greeting names the environment in words.
 */
@Component({
  selector: 'app-light-strip',
  template: `
    @for (l of lit(); track l.key) {
      <div class="lights" [class]="l.animation" [style.--light]="l.color">
        <span class="light"></span>
        @if (l.animation === 'flow') {
          <span class="light next"></span>
        }
      </div>
    }
    <canvas class="grain"></canvas>
  `,
  styleUrl: './light-strip.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { 'aria-hidden': 'true' },
})
export class LightStrip implements AfterViewInit {
  readonly settings = input.required<LightStripSettings>();

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef).nativeElement;
  private readonly destroyRef = inject(DestroyRef);

  /** The light, or none: a list of at most one, keyed so that a change starts it afresh. */
  protected readonly lit = computed(() => {
    const { color, animation } = this.settings();
    const light = lightOf(color);
    return light ? [{ key: `${color}/${animation}`, color: light, animation }] : [];
  });

  ngAfterViewInit(): void {
    // jsdom has no ResizeObserver; every browser Dash supports has.
    if (typeof ResizeObserver === 'undefined') {
      return;
    }
    const observer = new ResizeObserver(() => this.drawGrain());
    observer.observe(this.host);
    this.destroyRef.onDestroy(() => observer.disconnect());
  }

  /** Tiles the grain over the canvas at the display's density, again whenever the strip resizes. */
  private drawGrain(): void {
    const canvas = this.host.querySelector('canvas')!;
    const density = window.devicePixelRatio;
    const w = Math.round(canvas.clientWidth * density);
    const h = Math.round(canvas.clientHeight * density);
    const ctx = w > 0 && h > 0 ? canvas.getContext('2d') : null;
    if (!ctx) {
      return;
    }
    canvas.width = w;
    canvas.height = h;
    ctx.putImageData(new ImageData(tileGrain(w, h), w, h), 0, 0);
  }
}
