import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { Bookmark } from '../../api/models/bookmark';
import { BookmarkGroup } from '../../api/models/bookmark-group';
import { glyphs } from '../../design/glyphs';
import { Badge } from '../../shared/badge/badge';
import { GroupHead } from '../../shared/group-head/group-head';
import { ItemIcon } from '../../shared/item-icon/item-icon';
import { shownURL } from './tile';

/** A bookmark as a row shows it: its number across all groups and its address. */
interface Row {
  bookmark: Bookmark;
  index: string;
  url: string;
}

/**
 * The bookmarks (docs/adr/0012 D2): rows numbered across the groups, under group heads — a 20-pixel
 * icon, the name over its address, the badges as chips, an arrow. Under the pointer a row inverts.
 */
@Component({
  selector: 'app-bookmarks',
  imports: [GroupHead, ItemIcon, Badge],
  template: `
    @for (group of rows(); track group.name) {
      <section class="bookmark-group">
        @if (showGroups()) {
          <app-group-head [label]="group.name" [count]="group.rows.length" />
        }
        <ul>
          @for (row of group.rows; track row.bookmark.id) {
            <li>
              <a
                [href]="row.bookmark.url"
                [attr.target]="row.bookmark.target_blank ? '_blank' : null"
                [attr.rel]="row.bookmark.target_blank ? 'noopener noreferrer' : null"
              >
                <span class="index hud" aria-hidden="true">{{ row.index }}</span>
                <app-item-icon class="icon" [icon]="row.bookmark.icon" [size]="20" />
                <span class="text">
                  <span class="name">{{ row.bookmark.name }}</span>
                  <span class="url">{{ row.url }}</span>
                </span>
                @for (badge of row.bookmark.badges; track badge.corner) {
                  <app-badge [badge]="badge" [chip]="true" />
                }
                <svg class="arrow" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
                  <path [attr.d]="outward" />
                </svg>
              </a>
            </li>
          }
        </ul>
      </section>
    }
  `,
  styleUrl: './bookmarks.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Bookmarks {
  readonly groups = input.required<BookmarkGroup[]>();
  readonly showGroups = input(true);

  protected readonly outward = glyphs.outward;
  protected readonly rows = computed(() => {
    let n = 0;
    return this.groups().map((group) => ({
      name: group.name,
      rows: group.bookmarks.map((bookmark): Row => ({
        bookmark,
        index: String(++n).padStart(2, '0'),
        url: shownURL(bookmark.url),
      })),
    }));
  });
}
