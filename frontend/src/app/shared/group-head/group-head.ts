import { ChangeDetectionStrategy, Component, input } from '@angular/core';

/**
 * A group's head (docs/adr/0012 D2): its name as a label and its count at the far end, over a
 * hairline — an app group above its grid of tiles, a bookmark group above its rows.
 */
@Component({
  selector: 'app-group-head',
  template: `
    <h3 class="head hud">
      <span class="label">{{ label() }}</span>
      @if (count() !== undefined) {
        <span class="visually-hidden">, </span><span class="count">{{ count() }}</span>
      }
    </h3>
  `,
  styleUrl: './group-head.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class GroupHead {
  readonly label = input.required<string>();
  readonly count = input<number>();
}
