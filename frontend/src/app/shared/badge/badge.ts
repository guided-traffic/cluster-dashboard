import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { Badge as BadgeModel } from '../../api/models/badge';

/**
 * A badge placed as macOS places dock badges (docs/adr/0011), square as the page is (docs/adr/0012):
 * filled with its tone, its glyph or text in the tone's ink. On an icon it sits at its corner with a
 * third of it outside, cut from the icon by a ring in the surface's colour; as a chip it follows a
 * bookmark's name. Its label is the tooltip and what a screen reader says: a glyph alone says
 * nothing.
 */
@Component({
  selector: 'app-badge',
  template: `
    @if (badge().icon; as icon) {
      <span class="glyph" [style.--mask]="'url(&quot;' + icon.src + '&quot;)'"></span>
    } @else {
      <span class="text">{{ badge().text }}</span>
    }
  `,
  styleUrl: './badge.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    role: 'img',
    '[attr.aria-label]': 'badge().label',
    '[attr.title]': 'badge().label',
    '[class]': 'classes()',
  },
})
export class Badge {
  readonly badge = input.required<BadgeModel>();
  /** A chip in a line of text instead of a pill at a corner. */
  readonly chip = input(false);

  protected readonly classes = computed(() => {
    const b = this.badge();
    return `tone-${b.tone} ${this.chip() ? 'chip' : 'corner corner-' + b.corner}`;
  });
}
