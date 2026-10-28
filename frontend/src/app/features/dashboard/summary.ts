import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { AppStatus } from '../../api/models/app-status';
import { Summary as SummaryModel } from '../../api/models/summary';

/** A token of the summary: an app's state and the tile it links to. */
export interface SummaryToken {
  name: string;
  state: AppStatus['state'];
  anchor: string;
}

/** "13 apps in 3 groups.", or without the groups while they are not shown. */
export function leadWords(total: number, groups?: number): string {
  const apps = `${total} ${total === 1 ? 'app' : 'apps'}`;
  if (groups === undefined) {
    return `${apps}.`;
  }
  return `${apps} in ${groups} ${groups === 1 ? 'group' : 'groups'}.`;
}

/**
 * The overview's summary (docs/adr/0012 D2): a sentence naming the apps, their groups and what is
 * not healthy — worst first, each in its status colour — a square per app linking to its tile, and
 * the counts in a row of hairline cells. The squares are a picture for the eye; the sentence and
 * the counts are what a screen reader reads.
 */
@Component({
  selector: 'app-summary',
  template: `
    <div class="text">
      <p class="legend">
        {{ lead() }}
        @if (parts().length === 0) {
          All healthy.
        } @else {
          @for (part of parts(); track part.state; let last = $last) {
            <span class="part {{ part.state }}">{{ part.count }} {{ part.state }}</span
            >{{ last ? '.' : ',' }}
          }
        }
      </p>
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
    </div>
    <div class="counts-frame">
      <dl class="counts">
        @for (count of counts(); track count.state) {
          <div class="count {{ count.state }}" [class.zero]="count.value === 0">
            <dt class="hud">{{ count.label }}</dt>
            <dd>{{ count.value }}</dd>
          </div>
        }
      </dl>
    </div>
  `,
  styleUrl: './summary.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Summary {
  readonly summary = input.required<SummaryModel>();
  readonly tokens = input.required<SummaryToken[]>();
  /** How many groups the apps are in; undefined while ui.showAppGroups is off. */
  readonly groups = input<number>();

  protected readonly lead = computed(() => leadWords(this.summary().total, this.groups()));

  /** The states that are not healthy, worst first, and how many apps are in each. */
  protected readonly parts = computed(() => {
    const s = this.summary();
    return [
      { state: 'down', count: s.down },
      { state: 'degraded', count: s.degraded },
      { state: 'unknown', count: s.unknown },
    ].filter((part) => part.count > 0);
  });

  protected readonly counts = computed(() => {
    const s = this.summary();
    return [
      { state: 'total', label: 'Apps', value: s.total },
      { state: 'healthy', label: 'Healthy', value: s.healthy },
      { state: 'degraded', label: 'Degraded', value: s.degraded },
      { state: 'down', label: 'Down', value: s.down },
      { state: 'unknown', label: 'Unknown', value: s.unknown },
    ];
  });
}
