import { TestBed } from '@angular/core/testing';
import { LightStrip as LightStripSettings } from '../../api/models/light-strip';
import { grainTexture, LightStrip, lightOf, tileGrain } from './light-strip';

describe('LightStrip', () => {
  function render(settings: LightStripSettings) {
    const fixture = TestBed.createComponent(LightStrip);
    fixture.componentRef.setInput('settings', settings);
    fixture.detectChanges();
    return { fixture, host: fixture.nativeElement as HTMLElement };
  }

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('lights a name in its token, a #rrggbb as given, and nothing else', () => {
    expect(lightOf('red')).toBe('var(--strip-red)');
    expect(lightOf('green')).toBe('var(--strip-green)');
    expect(lightOf('#C0ffee')).toBe('#C0ffee');
    expect(lightOf('')).toBeUndefined();
    expect(lightOf('violet')).toBeUndefined();
    expect(lightOf('#c0ffee; background: url(x)')).toBeUndefined();
  });

  it('draws the specks Cogpit draws, from the same generator', () => {
    const px = grainTexture();
    const speck = (i: number) => [px[i * 4], px[i * 4 + 3]];
    // Reference values printed by Cogpit's own maths (lightstrip_model.go stripNoise, stripGrain).
    expect(speck(0)).toEqual([0, 4]);
    expect(speck(1)).toEqual([255, 10]);
    expect(speck(2)).toEqual([255, 1]);
    expect(speck(3)).toEqual([255, 11]);
    expect(speck(255)).toEqual([255, 3]);
    expect(speck(256)).toEqual([255, 7]);
    expect(speck(16383)).toEqual([0, 1]);
    let sum = 0;
    for (let i = 3; i < px.length; i += 4) {
      sum += px[i];
    }
    expect(sum).toBe(83372);
  });

  it('tiles the texture, a speck per device pixel', () => {
    const texture = grainTexture();
    const px = tileGrain(300, 65);
    const at = (data: Uint8ClampedArray, w: number, x: number, y: number) => [
      ...data.subarray((y * w + x) * 4, (y * w + x) * 4 + 4),
    ];
    expect(px.length).toBe(300 * 65 * 4);
    expect(at(px, 300, 256, 0)).toEqual(at(texture, 256, 0, 0));
    expect(at(px, 300, 299, 1)).toEqual(at(texture, 256, 43, 1));
    expect(at(px, 300, 3, 64)).toEqual(at(texture, 256, 3, 0));
  });

  it('is glass alone without a colour, and decorative', () => {
    const { host } = render({ color: '', animation: 'breathing' });
    expect(host.getAttribute('aria-hidden')).toBe('true');
    expect(host.querySelector('.lights')).toBeNull();
    expect(host.querySelector('canvas.grain')).not.toBeNull();
  });

  it('lights the colour and moves it as the animation says', () => {
    const { host } = render({ color: 'red', animation: 'breathing' });
    const lights = host.querySelector<HTMLElement>('.lights')!;
    expect([...lights.classList]).toEqual(['lights', 'breathing']);
    expect(lights.style.getPropertyValue('--light')).toBe('var(--strip-red)');
    expect(lights.querySelectorAll('.light').length).toBe(1);
  });

  it('runs a second light a pass behind in the flow', () => {
    const { host } = render({ color: '#c0ffee', animation: 'flow' });
    const lights = host.querySelector<HTMLElement>('.lights')!;
    expect(lights.classList.contains('flow')).toBe(true);
    expect(lights.style.getPropertyValue('--light')).toBe('#c0ffee');
    expect(lights.querySelectorAll('.light').length).toBe(2);
    expect(lights.querySelectorAll('.light.next').length).toBe(1);
  });

  it('starts the light afresh when its colour or animation changes', () => {
    const { fixture, host } = render({ color: 'red', animation: 'scanner' });
    const before = host.querySelector('.lights');
    fixture.componentRef.setInput('settings', { color: 'red', animation: 'scanner' });
    fixture.detectChanges();
    expect(host.querySelector('.lights')).toBe(before);
    fixture.componentRef.setInput('settings', { color: 'green', animation: 'scanner' });
    fixture.detectChanges();
    expect(host.querySelector('.lights')).not.toBe(before);
    expect(host.querySelector('.lights.scanner')).not.toBeNull();
  });

  it('tiles the grain over the canvas at the display density whenever the strip resizes', async () => {
    let resized: () => void = () => undefined;
    const disconnect = vi.fn();
    vi.stubGlobal(
      'ResizeObserver',
      class {
        constructor(callback: () => void) {
          resized = callback;
        }
        observe = vi.fn();
        disconnect = disconnect;
      },
    );
    vi.stubGlobal(
      'ImageData',
      class {
        constructor(
          readonly data: Uint8ClampedArray,
          readonly width: number,
          readonly height: number,
        ) {}
      },
    );
    vi.stubGlobal('devicePixelRatio', 2);
    const putImageData = vi.fn();
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
      putImageData,
    } as unknown as CanvasRenderingContext2D);

    const { fixture, host } = render({ color: '', animation: 'static' });
    await fixture.whenStable();
    const canvas = host.querySelector('canvas')!;
    Object.defineProperty(canvas, 'clientWidth', { value: 300 });
    Object.defineProperty(canvas, 'clientHeight', { value: 4 });
    resized();

    expect(canvas.width).toBe(600);
    expect(canvas.height).toBe(8);
    const [image, x, y] = putImageData.mock.calls[0];
    expect([image.width, image.height, x, y]).toEqual([600, 8, 0, 0]);
    expect(image.data).toEqual(tileGrain(600, 8));

    fixture.destroy();
    expect(disconnect).toHaveBeenCalled();
  });

  it('draws no grain while the strip has no size', async () => {
    let resized: () => void = () => undefined;
    vi.stubGlobal(
      'ResizeObserver',
      class {
        constructor(callback: () => void) {
          resized = callback;
        }
        observe = vi.fn();
        disconnect = vi.fn();
      },
    );
    const getContext = vi.spyOn(HTMLCanvasElement.prototype, 'getContext');
    const { fixture } = render({ color: 'blue', animation: 'static' });
    await fixture.whenStable();
    resized();
    expect(getContext).not.toHaveBeenCalled();
  });
});
