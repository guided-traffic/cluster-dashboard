import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { Icon } from '../../api/models/icon';
import { ThemeService } from '../../core/theme.service';

/**
 * An item's icon as the backend resolved it (docs/adr/0008 D3): a monochrome glyph is a CSS mask
 * filled with the text colour, so it follows the scheme and lights up with its tile; a logo is an
 * image, its dark variant in the dark scheme. Decorative: the item's name labels it. An SVG is
 * only ever drawn as a mask or an image, where its scripts never run (docs/security/served-icons.md).
 */
@Component({
  selector: 'app-item-icon',
  template: `
    @if (icon().tint) {
      <span class="glyph" [style.--mask]="mask()"></span>
    } @else {
      <img [src]="src()" alt="" decoding="async" draggable="false" />
    }
  `,
  styleUrl: './item-icon.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { 'aria-hidden': 'true', '[style.--icon-size.px]': 'size()' },
})
export class ItemIcon {
  private readonly theme = inject(ThemeService);

  readonly icon = input.required<Icon>();
  /** The side in CSS pixels: 40 on a tile, 20 in a bookmark row. */
  readonly size = input(40);

  protected readonly src = computed(() => {
    const icon = this.icon();
    return this.theme.scheme() === 'dark' && icon.src_dark ? icon.src_dark : icon.src;
  });
  protected readonly mask = computed(() => `url("${this.src()}")`);
}
