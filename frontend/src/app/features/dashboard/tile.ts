import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { App } from '../../api/models/app';
import { glyphs } from '../../design/glyphs';
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

/** The status in words: the ready count when healthy, quiet; none when unknown. */
export function statusWord(app: App): string | undefined {
  switch (app.status.state) {
    case 'healthy':
      return `${app.status.ready}/${app.status.total}`;
    case 'degraded':
      return `${app.status.ready} of ${app.status.total} ready`;
    case 'down':
      return 'down';
    default:
      return undefined;
  }
}

/**
 * An app's tile (docs/adr/0012 D3): a cell of the group's hairline grid — the icon in a square
 * frame with its badges, the status at the far end, the name, the description and the address.
 * Quiet when healthy, the ready count in faint text; degraded and down draw a line along the top
 * edge and say it in words — colour is never the only signal. Under the pointer the cell lifts a
 * step and a glyph's frame inverts.
 */
@Component({
  selector: 'app-tile',
  imports: [ItemIcon, Badge],
  template: `
    <a
      class="tile"
      [href]="app().url"
      [attr.target]="app().target_blank ? '_blank' : null"
      [attr.rel]="app().target_blank ? 'noopener noreferrer' : null"
      [class.degraded]="app().status.state === 'degraded'"
      [class.down]="app().status.state === 'down'"
    >
      <span class="icon">
        <app-item-icon [icon]="app().icon" [size]="28" />
      </span>
      <!-- The text first, placed by the grid: the link's name begins with the app's. -->
      <span class="name">{{ app().name }}</span>
      @if (word(); as word) {
        <span class="status hud">{{ word }}</span>
      }
      @if (app().info) {
        <span class="info">{{ app().info }}</span>
      }
      @if (showURL()) {
        <span class="url"
          ><span class="host">{{ url() }}</span
          ><svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
            <path [attr.d]="outward" /></svg
        ></span>
      }
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
  readonly showURL = input(true);

  protected readonly url = computed(() => shownURL(this.app().url));
  protected readonly word = computed(() => statusWord(this.app()));
  protected readonly outward = glyphs.outward;
}
