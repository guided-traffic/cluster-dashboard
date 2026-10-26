import { ChangeDetectionStrategy, Component, input } from '@angular/core';

/**
 * Cogpit's bracket head (glow.go's bracketLabel): a lowercase label with a middle dot and a count,
 * above a hairline with end ticks in the label's light at about 55 %. Cyan is the cluster's light:
 * the summary and the app groups; muted for the bookmarks, which are the person's.
 */
@Component({
  selector: 'app-bracket',
  template: `
    <h2 class="label">
      {{ label() }}
      @if (count() !== undefined) {
        <span class="dot" aria-hidden="true">·</span><span class="visually-hidden">,</span>
        {{ count() }}
      }
    </h2>
    <div class="rule" aria-hidden="true"></div>
  `,
  styleUrl: './bracket.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '[class.muted]': "tone() === 'muted'" },
})
export class Bracket {
  readonly label = input.required<string>();
  readonly count = input<number>();
  readonly tone = input<'cyan' | 'muted'>('cyan');
}
