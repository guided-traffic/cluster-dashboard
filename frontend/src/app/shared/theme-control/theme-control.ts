import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { ThemePreference, ThemeService } from '../../core/theme.service';
import { glyphs } from '../../design/glyphs';

interface ThemeOption {
  value: ThemePreference;
  label: string;
  glyph: string;
}

/**
 * Cogpit's segmented control for the colour scheme (docs/adr/0005): one plate, three segments as
 * Material Symbols glyphs, the chosen one a step lighter with a glow-blue underline. It is a group
 * of native radio buttons under a legend, so a screen reader names the group and each segment,
 * and the arrow keys move the choice as they do in any radio group.
 */
@Component({
  selector: 'app-theme-control',
  templateUrl: './theme-control.html',
  styleUrl: './theme-control.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ThemeControl {
  protected readonly theme = inject(ThemeService);
  protected readonly options: readonly ThemeOption[] = [
    { value: 'system', label: 'System', glyph: glyphs.system },
    { value: 'light', label: 'Light', glyph: glyphs.light },
    { value: 'dark', label: 'Dark', glyph: glyphs.dark },
  ];
}
