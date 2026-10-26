import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { App } from '../../api/models/app';
import { Badge } from '../../shared/badge/badge';
import { ItemIcon } from '../../shared/item-icon/item-icon';

/** The address as a tile shows it: the host, and the path when it is more than "/". */
export function shownURL(url: string): string {
  try {
    const u = new URL(url);
    const path = u.pathname === '/' ? '' : u.pathname;
    return u.host + path;
  } catch {
    return url;
  }
}

/** The word of a status a tile lights up for: none for healthy and unknown. */
export function statusWord(app: App): string | undefined {
  switch (app.status.state) {
    case 'degraded':
      return `${app.status.ready} of ${app.status.total} ready`;
    case 'down':
      return 'down';
    default:
      return undefined;
  }
}

/**
 * An app's tile (docs/adr/0005, Cogpit's plates): the icon with its badges, the name, the address
 * and the description. Quiet when healthy; degraded and down light up with a spine at the left
 * edge and a pill that says it in words — glow is never the only signal. Hover and keyboard focus
 * draw Cogpit's glow frame in glow blue.
 */
@Component({
  selector: 'app-tile',
  imports: [ItemIcon, Badge],
  template: `
    <a
      class="tile"
      [id]="anchor()"
      [href]="app().url"
      [attr.target]="app().target_blank ? '_blank' : null"
      [attr.rel]="app().target_blank ? 'noopener noreferrer' : null"
      [class.degraded]="app().status.state === 'degraded'"
      [class.down]="app().status.state === 'down'"
    >
      <span class="icon">
        <app-item-icon [icon]="app().icon" [size]="40" />
      </span>
      <span class="text">
        <span class="name">{{ app().name }}</span>
        @if (showURL()) {
          <span class="url">{{ url() }}</span>
        }
        @if (app().info) {
          <span class="info">{{ app().info }}</span>
        }
        @if (word(); as word) {
          <span class="pill"><span class="dot" aria-hidden="true"></span>{{ word }}</span>
        }
      </span>
      <!-- Over the icon, but after the text: the link's name begins with the app's. -->
      @if (app().badges.length > 0) {
        <span class="badges">
          @for (badge of app().badges; track badge.corner) {
            <app-badge [badge]="badge" />
          }
        </span>
      }
    </a>
  `,
  styleUrl: './tile.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Tile {
  readonly app = input.required<App>();
  /** The id the summary's token links to. */
  readonly anchor = input<string>();
  readonly showURL = input(true);

  protected readonly url = computed(() => shownURL(this.app().url));
  protected readonly word = computed(() => statusWord(this.app()));
}
