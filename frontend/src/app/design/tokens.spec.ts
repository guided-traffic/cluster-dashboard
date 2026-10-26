import tokensCss from './tokens.css' with { loader: 'text' };

/** One token: its light and its dark value. */
interface Pair {
  light: string;
  dark: string;
}

/** Every `--name: light-dark(#light, #dark);` of tokens.css. */
function readTokens(css: string): Map<string, Pair> {
  const tokens = new Map<string, Pair>();
  for (const m of css.matchAll(/--([a-z-]+):\s*light-dark\((#[0-9a-f]{6}),\s*(#[0-9a-f]{6})\)/g)) {
    tokens.set(m[1], { light: m[2], dark: m[3] });
  }
  return tokens;
}

/** WCAG 2.x relative luminance of #rrggbb. */
function luminance(hex: string): number {
  const channel = (i: number) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * channel(1) + 0.7152 * channel(3) + 0.0722 * channel(5);
}

/** The WCAG contrast ratio of two colours. */
function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

const tokens = readTokens(tokensCss);
const schemes = ['light', 'dark'] as const;
const surfaces = ['stage', 'chrome', 'plate', 'plate-lifted', 'header'];
/** Read as text: 4.5:1 on every surface (WCAG 1.4.3). */
const texts = ['text', 'muted', 'link'];
/** Seen as shapes — a focus ring, an underline, a status spine, a glyph: 3:1 (WCAG 1.4.11). */
const shapes = ['glow', 'cyan', 'mint', 'amber', 'orange', 'coral', 'text', 'muted'];

describe('the design tokens', () => {
  it('are read from tokens.css, each a light and a dark value', () => {
    expect([...tokens.keys()]).toEqual(
      expect.arrayContaining([...surfaces, ...texts, ...shapes, 'hairline', 'edge', 'outline']),
    );
  });

  it('keep the values of Cogpit’s dark palette named in the plan', () => {
    expect(tokens.get('stage')?.dark).toBe('#0d121a');
    expect(tokens.get('chrome')?.dark).toBe('#0a0e14');
    expect(tokens.get('text')?.dark).toBe('#d5dde8');
    expect(tokens.get('glow')?.dark).toBe('#4aa8ff');
    expect(tokens.get('cyan')?.dark).toBe('#5ad1e3');
    expect(tokens.get('coral')?.dark).toBe('#ff6b74');
  });

  for (const scheme of schemes) {
    for (const surface of surfaces) {
      for (const text of texts) {
        it(`hold ${text} on ${surface} to 4.5:1 in the ${scheme} scheme`, () => {
          const ratio = contrast(tokens.get(text)![scheme], tokens.get(surface)![scheme]);
          expect(ratio).toBeGreaterThanOrEqual(4.5);
        });
      }
      for (const shape of shapes) {
        it(`hold ${shape} on ${surface} to 3:1 in the ${scheme} scheme`, () => {
          const ratio = contrast(tokens.get(shape)![scheme], tokens.get(surface)![scheme]);
          expect(ratio).toBeGreaterThanOrEqual(3);
        });
      }
    }
  }

  const tones = ['neutral', 'blue', 'cyan', 'mint', 'amber', 'coral'];
  for (const scheme of schemes) {
    for (const status of ['amber', 'coral']) {
      it(`hold the ${status} status word on its tint to 4.5:1 in the ${scheme} scheme`, () => {
        const ratio = contrast(
          tokens.get(`${status}-word`)![scheme],
          tokens.get(`${status}-tint`)![scheme],
        );
        expect(ratio).toBeGreaterThanOrEqual(4.5);
      });
    }
    for (const tone of tones) {
      it(`hold the ${tone} badge's ink to 4.5:1 and its fill to 3:1 on every surface in the ${scheme} scheme`, () => {
        const fill = tokens.get(`tone-${tone}`)![scheme];
        expect(contrast(tokens.get(`tone-${tone}-ink`)![scheme], fill)).toBeGreaterThanOrEqual(4.5);
        for (const surface of surfaces) {
          expect(contrast(fill, tokens.get(surface)![scheme])).toBeGreaterThanOrEqual(3);
        }
      });
    }
  }

  it('have no violet tone: violet is the agent’s light in Cogpit', () => {
    expect([...tokens.keys()].some((name) => name.includes('violet'))).toBe(false);
  });

  it('compute the WCAG ratio', () => {
    expect(contrast('#000000', '#ffffff')).toBeCloseTo(21, 5);
    expect(contrast('#777777', '#777777')).toBeCloseTo(1, 5);
  });
});
