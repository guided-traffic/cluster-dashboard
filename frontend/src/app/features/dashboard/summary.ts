import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { AppStatus } from '../../api/models/app-status';
import { Summary as SummaryModel } from '../../api/models/summary';
import { Bracket } from '../../shared/bracket/bracket';

/** A token of the summary: an app's state and the tile it links to. */
export interface SummaryToken {
  name: string;
  state: AppStatus['state'];
  anchor: string;
}

/**
 * Cogpit's health header for the dashboard: `apps · 42` in the cyan, a rounded token per app —
 * healthy at about a third of its light, degraded amber, down coral, unknown hollow — and a legend
 * that names only what is not healthy. A token links to its tile. The tokens are a picture for the
 * eye; the legend is what a screen reader reads.
 */
@Component({
  selector: 'app-summary',
  imports: [Bracket],
  template: `
    <app-bracket label="apps" [count]="summary().total" />
    <div class="tokens" aria-hidden="true">
      @for (token of tokens(); track token.anchor) {
        <a
          class="token {{ token.state }}"
          [href]="'#' + token.anchor"
          [title]="token.name"
          tabindex="-1"
        ></a>
      }
    </div>
    <p class="legend">
      @if (legend().length === 0) {
        all healthy
      } @else {
        @for (part of legend(); track part.state; let last = $last) {
          <span class="part {{ part.state }}">{{ part.count }} {{ part.state }}</span>
          @if (!last) {
            <span class="sep" aria-hidden="true">·</span><span class="visually-hidden">, </span>
          }
        }
      }
    </p>
  `,
  styleUrl: './summary.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Summary {
  readonly summary = input.required<SummaryModel>();
  readonly tokens = input.required<SummaryToken[]>();

  /** The states that are not healthy, worst first, and how many apps are in each. */
  protected readonly legend = computed(() => {
    const s = this.summary();
    return [
      { state: 'down', count: s.down },
      { state: 'degraded', count: s.degraded },
      { state: 'unknown', count: s.unknown },
    ].filter((part) => part.count > 0);
  });
}
