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
/** Where text and shapes stand: the page, a tile under the pointer, a down tile. */
const surfaces = ['bg', 'raised', 'down-tint'];
/** Read as text — the status colours too, as words and counts: 4.5:1 on every surface (WCAG 1.4.3). */
const texts = ['ink', 'strong', 'muted', 'faint', 'amber', 'coral'];
/** Seen as shapes — the focus ring, a status line, a healthy app's square, a glyph: 3:1 (WCAG 1.4.11). */
const shapes = ['accent', 'amber', 'coral', 'line-strong', 'ink', 'muted'];

const ratio = (a: string, b: string, scheme: (typeof schemes)[number]) =>
  contrast(tokens.get(a)![scheme], tokens.get(b)![scheme]);

describe('the design tokens', () => {
  it('are read from tokens.css, each a light and a dark value', () => {
    expect([...tokens.keys()]).toEqual(
      expect.arrayContaining([
        ...surfaces,
        ...texts,
        ...shapes,
        'line',
        'stale-tint',
        'invert-bg',
        'invert-fg',
        'accent-invert',
      ]),
    );
  });

  it('keep the grounds of the look (docs/adr/0012 D1)', () => {
    expect(tokens.get('bg')).toEqual({ light: '#f2f1ee', dark: '#121316' });
    expect(tokens.get('ink')).toEqual({ light: '#101014', dark: '#f2f1ee' });
  });

  for (const scheme of schemes) {
    for (const surface of surfaces) {
      for (const text of texts) {
        it(`hold ${text} on ${surface} to 4.5:1 in the ${scheme} scheme`, () => {
          expect(ratio(text, surface, scheme)).toBeGreaterThanOrEqual(4.5);
        });
      }
      for (const shape of shapes) {
        it(`hold ${shape} on ${surface} to 3:1 in the ${scheme} scheme`, () => {
          expect(ratio(shape, surface, scheme)).toBeGreaterThanOrEqual(3);
        });
      }
    }

    it(`hold the stale notice's word on its tint to 4.5:1 in the ${scheme} scheme`, () => {
      expect(ratio('amber', 'stale-tint', scheme)).toBeGreaterThanOrEqual(4.5);
    });

    it(`hold the inverted text to 4.5:1 and its focus ring to 3:1 in the ${scheme} scheme`, () => {
      expect(ratio('invert-fg', 'invert-bg', scheme)).toBeGreaterThanOrEqual(4.5);
      expect(ratio('accent-invert', 'invert-bg', scheme)).toBeGreaterThanOrEqual(3);
    });
  }

  const tones = ['neutral', 'blue', 'cyan', 'mint', 'amber', 'coral'];
  for (const scheme of schemes) {
    for (const tone of tones) {
      it(`hold the ${tone} badge's ink to 4.5:1 and its fill to 3:1 on every surface in the ${scheme} scheme`, () => {
        expect(ratio(`tone-${tone}-ink`, `tone-${tone}`, scheme)).toBeGreaterThanOrEqual(4.5);
        for (const surface of surfaces) {
          expect(ratio(`tone-${tone}`, surface, scheme)).toBeGreaterThanOrEqual(3);
        }
      });
    }
  }

  it('have no violet tone: the backend refuses violet as a badge tone (internal/badges)', () => {
    expect([...tokens.keys()].some((name) => name.includes('violet'))).toBe(false);
  });

  it('compute the WCAG ratio', () => {
    expect(contrast('#000000', '#ffffff')).toBeCloseTo(21, 5);
    expect(contrast('#777777', '#777777')).toBeCloseTo(1, 5);
  });
});
