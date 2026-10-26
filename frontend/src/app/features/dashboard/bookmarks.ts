import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { BookmarkGroup } from '../../api/models/bookmark-group';
import { Badge } from '../../shared/badge/badge';
import { Bracket } from '../../shared/bracket/bracket';
import { ItemIcon } from '../../shared/item-icon/item-icon';

/**
 * The bookmarks: compact rows under bracket heads in the muted light — the person's links, not the
 * cluster's — a 20-pixel icon and the name, a quiet link that turns glow blue with an underline
 * under the pointer. Badges become small chips after the name.
 */
@Component({
  selector: 'app-bookmarks',
  imports: [Bracket, ItemIcon, Badge],
  template: `
    @for (group of groups(); track group.name) {
      <section class="bookmark-group">
        @if (showGroups()) {
          <app-bracket [label]="group.name" [count]="group.bookmarks.length" tone="muted" />
        }
        <ul>
          @for (bookmark of group.bookmarks; track bookmark.id) {
            <li>
              <a
                [href]="bookmark.url"
                [attr.target]="bookmark.target_blank ? '_blank' : null"
                [attr.rel]="bookmark.target_blank ? 'noopener noreferrer' : null"
              >
                <app-item-icon class="icon" [icon]="bookmark.icon" [size]="20" />
                <span class="name">{{ bookmark.name }}</span>
                @for (badge of bookmark.badges; track badge.corner) {
                  <app-badge [badge]="badge" [chip]="true" />
                }
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
}
